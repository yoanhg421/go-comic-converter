/*
Tools to create EPUB from images.
*/
package epub

import (
	"archive/zip"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	epubimage "github.com/celogeek/go-comic-converter/v2/internal/epub/image"
	epubimageprocessor "github.com/celogeek/go-comic-converter/v2/internal/epub/imageprocessor"
	epuboptions "github.com/celogeek/go-comic-converter/v2/internal/epub/options"
	epubprogress "github.com/celogeek/go-comic-converter/v2/internal/epub/progress"
	epubtemplates "github.com/celogeek/go-comic-converter/v2/internal/epub/templates"
	epubtree "github.com/celogeek/go-comic-converter/v2/internal/epub/tree"
	epubzip "github.com/celogeek/go-comic-converter/v2/internal/epub/zip"
	"github.com/gofrs/uuid"
)

type ePub struct {
	*epuboptions.Options
	UID       string
	Publisher string
	UpdatedAt string

	templateProcessor *template.Template
	imageProcessor    *epubimageprocessor.EPUBImageProcessor
}

type epubPart struct {
	Cover  *epubimage.Image
	Images []*epubimage.Image
	Reader *zip.ReadCloser
}

// initialize EPUB
func New(options *epuboptions.Options) *ePub {
	uid := uuid.Must(uuid.NewV4())
	tmpl := template.New("parser")
	tmpl.Funcs(template.FuncMap{
		"mod":  func(i, j int) bool { return i%j == 0 },
		"zoom": func(s int, z float32) int { return int(float32(s) * z) },
	})

	return &ePub{
		Options:           options,
		UID:               uid.String(),
		Publisher:         "GO Comic Converter",
		UpdatedAt:         time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		templateProcessor: tmpl,
		imageProcessor:    epubimageprocessor.New(options),
	}
}

// render templates
func (e *ePub) render(templateString string, data map[string]any) string {
	var result strings.Builder
	tmpl := template.Must(e.templateProcessor.Parse(templateString))
	if err := tmpl.Execute(&result, data); err != nil {
		panic(err)
	}
	return regexp.MustCompile("\n+").ReplaceAllString(result.String(), "\n")
}

// write image to the zip
func (e *ePub) writeImage(wz *epubzip.EPUBZip, img *epubimage.Image, zipImg *zip.File) error {
	err := wz.WriteContent(
		img.EPUBPagePath(),
		[]byte(e.render(epubtemplates.Text, map[string]any{
			"Title":      fmt.Sprintf("Image %d Part %d", img.Id, img.Part),
			"ViewPort":   fmt.Sprintf("width=%d,height=%d", e.Image.View.Width, e.Image.View.Height),
			"ImagePath":  img.ImgPath(),
			"ImageStyle": img.ImgStyle(e.Image.View.Width, e.Image.View.Height, ""),
		})),
	)
	if err == nil {
		err = wz.Copy(zipImg)
	}

	return err
}

// write blank page
func (e *ePub) writeBlank(wz *epubzip.EPUBZip, img *epubimage.Image) error {
	return wz.WriteContent(
		img.EPUBSpacePath(),
		[]byte(e.render(epubtemplates.Blank, map[string]any{
			"Title":    fmt.Sprintf("Blank Page %d", img.Id),
			"ViewPort": fmt.Sprintf("width=%d,height=%d", e.Image.View.Width, e.Image.View.Height),
		})),
	)
}

// write title image
func (e *ePub) writeCoverImage(wz *epubzip.EPUBZip, img *epubimage.Image, part, totalParts int) error {
	title := "Cover"
	text := ""
	if totalParts > 1 {
		text = fmt.Sprintf("%d / %d", part, totalParts)
		title = fmt.Sprintf("%s %s", title, text)
	}

	if err := wz.WriteContent(
		"OEBPS/Text/cover.xhtml",
		[]byte(e.render(epubtemplates.Text, map[string]any{
			"Title":      title,
			"ViewPort":   fmt.Sprintf("width=%d,height=%d", e.Image.View.Width, e.Image.View.Height),
			"ImagePath":  fmt.Sprintf("Images/cover.%s", e.Image.Format),
			"ImageStyle": img.ImgStyle(e.Image.View.Width, e.Image.View.Height, ""),
		})),
	); err != nil {
		return err
	}

	coverTitle, err := e.imageProcessor.CoverTitleData(&epubimageprocessor.CoverTitleDataOptions{
		Src:         img.Raw,
		Name:        "cover",
		Text:        text,
		Align:       "bottom",
		PctWidth:    50,
		PctMargin:   50,
		MaxFontSize: 96,
		BorderSize:  8,
	})

	if err != nil {
		return err
	}

	if err := wz.WriteRaw(coverTitle); err != nil {
		return err
	}

	return nil
}

// write title image
func (e *ePub) writeTitleImage(wz *epubzip.EPUBZip, img *epubimage.Image, title string) error {
	titleAlign := ""
	if !e.Image.View.PortraitOnly {
		if e.Image.Manga {
			titleAlign = "right:0"
		} else {
			titleAlign = "left:0"
		}
	}

	if !e.Image.View.PortraitOnly {
		if err := wz.WriteContent(
			"OEBPS/Text/space_title.xhtml",
			[]byte(e.render(epubtemplates.Blank, map[string]any{
				"Title":    "Blank Page Title",
				"ViewPort": fmt.Sprintf("width=%d,height=%d", e.Image.View.Width, e.Image.View.Height),
			})),
		); err != nil {
			return err
		}
	}

	if err := wz.WriteContent(
		"OEBPS/Text/title.xhtml",
		[]byte(e.render(epubtemplates.Text, map[string]any{
			"Title":      title,
			"ViewPort":   fmt.Sprintf("width=%d,height=%d", e.Image.View.Width, e.Image.View.Height),
			"ImagePath":  fmt.Sprintf("Images/title.%s", e.Image.Format),
			"ImageStyle": img.ImgStyle(e.Image.View.Width, e.Image.View.Height, titleAlign),
		})),
	); err != nil {
		return err
	}

	coverTitle, err := e.imageProcessor.CoverTitleData(&epubimageprocessor.CoverTitleDataOptions{
		Src:         img.Raw,
		Name:        "title",
		Text:        title,
		Align:       "center",
		PctWidth:    100,
		PctMargin:   100,
		MaxFontSize: 64,
		BorderSize:  4,
	})
	if err != nil {
		return err
	}

	if err := wz.WriteRaw(coverTitle); err != nil {
		return err
	}

	return nil
}

// extract image and split it into part
func (e *ePub) getParts() (parts []*epubPart, imgStorage *epubzip.EPUBZipStorageImageReader, err error) {
	images, err := e.imageProcessor.Load()

	if err != nil {
		return nil, nil, err
	}

	// sort result by id and part
	sort.Slice(images, func(i, j int) bool {
		if images[i].Id == images[j].Id {
			return images[i].Part < images[j].Part
		}
		return images[i].Id < images[j].Id
	})

	parts = make([]*epubPart, 0)
	cover := images[0]
	if e.Image.HasCover {
		images = images[1:]
	}

	if e.Dry {
		parts = append(parts, &epubPart{
			Cover:  cover,
			Images: images,
		})
		return parts, nil, nil
	}

	imgStorage, err = epubzip.NewEPUBZipStorageImageReader(e.ImgStorage())
	if err != nil {
		return nil, nil, err
	}

	// compute size of the EPUB part and try to be as close as possible of the target
	maxSize := uint64(e.LimitMb * 1024 * 1024)
	xhtmlSize := uint64(1024)
	// descriptor files + title + cover
	baseSize := uint64(128*1024) + imgStorage.Size(cover.EPUBImgPath())*2

	currentSize := baseSize
	currentImages := make([]*epubimage.Image, 0)
	part := 1

	for _, img := range images {
		imgSize := imgStorage.Size(img.EPUBImgPath()) + xhtmlSize
		if maxSize > 0 && len(currentImages) > 0 && currentSize+imgSize > maxSize {
			parts = append(parts, &epubPart{
				Cover:  cover,
				Images: currentImages,
			})
			part += 1
			currentSize = baseSize
			currentImages = make([]*epubimage.Image, 0)
		}
		currentSize += imgSize
		currentImages = append(currentImages, img)
	}
	if len(currentImages) > 0 {
		parts = append(parts, &epubPart{
			Cover:  cover,
			Images: currentImages,
		})
	}

	return parts, imgStorage, nil
}

// create a tree from the directories.
//
// this is used to simulate the toc.
func (e *ePub) getTree(images []*epubimage.Image, skip_files bool) string {
	t := epubtree.New()
	for _, img := range images {
		if skip_files {
			t.Add(img.Path)
		} else {
			t.Add(filepath.Join(img.Path, img.Name))
		}
	}
	c := t.Root()
	if skip_files && e.StripFirstDirectoryFromToc && len(c.Children) == 1 {
		c = c.Children[0]
	}

	return c.WriteString("")
}

func (e *ePub) computeAspectRatio(epubParts []*epubPart) float64 {
	var (
		bestAspectRatio      float64
		bestAspectRatioCount int
		aspectRatio          = map[float64]int{}
	)

	trunc := func(v float64) float64 {
		return float64(math.Round(v*10000)) / 10000
	}

	for _, p := range epubParts {
		aspectRatio[trunc(p.Cover.OriginalAspectRatio)]++
		for _, i := range p.Images {
			aspectRatio[trunc(i.OriginalAspectRatio)]++
		}
	}

	for k, v := range aspectRatio {
		if v > bestAspectRatioCount {
			bestAspectRatio, bestAspectRatioCount = k, v
		}
	}

	return bestAspectRatio
}

func (e *ePub) computeViewPort(epubParts []*epubPart) {
	if e.Image.View.AspectRatio == -1 {
		return //keep device size
	}

	// readjusting view port
	bestAspectRatio := e.Image.View.AspectRatio
	if bestAspectRatio == 0 {
		bestAspectRatio = e.computeAspectRatio(epubParts)
	}

	viewWidth, viewHeight := int(float64(e.Image.View.Height)/bestAspectRatio), int(float64(e.Image.View.Width)*bestAspectRatio)
	if viewWidth > e.Image.View.Width {
		e.Image.View.Height = viewHeight
	} else {
		e.Image.View.Width = viewWidth
	}
}

// create the zip
func (e *ePub) Write() error {
	type zipContent struct {
		Name    string
		Content string
	}

	epubParts, imgStorage, err := e.getParts()
	if err != nil {
		return err
	}

	if e.Dry {
		p := epubParts[0]
		fmt.Fprintf(os.Stderr, "TOC:\n  - %s\n%s\n", e.Title, e.getTree(p.Images, true))
		if e.DryVerbose {
			if e.Image.HasCover {
				fmt.Fprintf(os.Stderr, "Cover:\n%s\n", e.getTree([]*epubimage.Image{p.Cover}, false))
			}
			fmt.Fprintf(os.Stderr, "Files:\n%s\n", e.getTree(p.Images, false))
		}
		return nil
	}
	defer func() {
		imgStorage.Close()
		imgStorage.Remove()
	}()

	totalParts := len(epubParts)

	bar := epubprogress.New(epubprogress.Options{
		Max:         totalParts,
		Description: "Writing Part",
		CurrentJob:  2,
		TotalJob:    2,
		Quiet:       e.Quiet,
		Json:        e.Json,
	})

	e.computeViewPort(epubParts)
	hasTitlePage := e.TitlePage == 1 || (e.TitlePage == 2 && totalParts > 1)
	for i, part := range epubParts {
		ext := filepath.Ext(e.Output)
		suffix := ""
		if totalParts > 1 {
			fmtLen := len(fmt.Sprint(totalParts))
			fmtPart := fmt.Sprintf(" Part %%0%dd of %%0%dd", fmtLen, fmtLen)
			suffix = fmt.Sprintf(fmtPart, i+1, totalParts)
		}

		path := fmt.Sprintf("%s%s%s", e.Output[0:len(e.Output)-len(ext)], suffix, ext)
		wz, err := epubzip.New(path)
		if err != nil {
			return err
		}
		defer wz.Close()

		title := e.Title
		if totalParts > 1 {
			title = fmt.Sprintf("%s [%d/%d]", title, i+1, totalParts)
		}

		content := []zipContent{
			{"META-INF/container.xml", epubtemplates.Container},
			{"META-INF/com.apple.ibooks.display-options.xml", epubtemplates.AppleBooks},
			{"OEBPS/content.opf", epubtemplates.Content(&epubtemplates.ContentOptions{
				Title:        title,
				HasTitlePage: hasTitlePage,
				UID:          e.UID,
				Author:       e.Author,
				Publisher:    e.Publisher,
				UpdatedAt:    e.UpdatedAt,
				ImageOptions: e.Image,
				Cover:        part.Cover,
				Images:       part.Images,
				Current:      i + 1,
				Total:        totalParts,
			})},
			{"OEBPS/toc.xhtml", epubtemplates.Toc(title, hasTitlePage, e.StripFirstDirectoryFromToc, part.Images)},
			{"OEBPS/Text/style.css", e.render(epubtemplates.Style, map[string]any{
				"View": e.Image.View,
			})},
		}

		if err = wz.WriteMagic(); err != nil {
			return err
		}
		for _, c := range content {
			if err := wz.WriteContent(c.Name, []byte(c.Content)); err != nil {
				return err
			}
		}

		if err = e.writeCoverImage(wz, part.Cover, i+1, totalParts); err != nil {
			return err
		}

		if hasTitlePage {
			if err = e.writeTitleImage(wz, part.Cover, title); err != nil {
				return err
			}
		}

		lastImage := part.Images[len(part.Images)-1]
		for _, img := range part.Images {
			if err := e.writeImage(wz, img, imgStorage.Get(img.EPUBImgPath())); err != nil {
				return err
			}

			// Double Page or Last Image that is not a double page
			if !e.Image.View.PortraitOnly &&
				(img.DoublePage ||
					(!e.Image.KeepDoublePageIfSplitted && img.Part == 1) ||
					(img.Part == 0 && img == lastImage)) {
				if err := e.writeBlank(wz, img); err != nil {
					return err
				}
			}
		}
		bar.Add(1)
	}
	bar.Close()
	if !e.Json {
		fmt.Fprintln(os.Stderr)
	}

	// display corrupted images
	hasError := false
	for pId, part := range epubParts {
		if pId == 0 && e.Image.HasCover && part.Cover.Error != nil {
			hasError = true
			fmt.Fprintf(os.Stderr, "Error on image %s: %v\n", filepath.Join(part.Cover.Path, part.Cover.Name), part.Cover.Error)
		}
		for _, img := range part.Images {
			if img.Part == 0 && img.Error != nil {
				hasError = true
				fmt.Fprintf(os.Stderr, "Error on image %s: %v\n", filepath.Join(img.Path, img.Name), img.Error)
			}
		}
	}
	if hasError {
		fmt.Fprintln(os.Stderr)
	}

	return nil
}
