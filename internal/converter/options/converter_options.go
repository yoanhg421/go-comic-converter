/*
Manage options with default value from config.
*/
package options

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/celogeek/go-comic-converter/v2/internal/converter/profiles"
	"gopkg.in/yaml.v3"
)

type Options struct {
	// Output
	Input  string `yaml:"-"`
	Output string `yaml:"-"`
	Author string `yaml:"-"`
	Title  string `yaml:"-"`

	// Config
	Profile                    string `yaml:"profile"`
	Quality                    int    `yaml:"quality"`
	Crop                       bool   `yaml:"crop"`
	CropRatioLeft              int    `yaml:"crop_ratio_left"`
	CropRatioUp                int    `yaml:"crop_ratio_up"`
	CropRatioRight             int    `yaml:"crop_ratio_right"`
	CropRatioBottom            int    `yaml:"crop_ratio_bottom"`
	Brightness                 int    `yaml:"brightness"`
	Contrast                   int    `yaml:"contrast"`
	Auto                       bool   `yaml:"-"`
	AutoRotate                 bool   `yaml:"auto_rotate"`
	AutoSplitDoublePage        bool   `yaml:"auto_split_double_page"`
	NoBlankImage               bool   `yaml:"no_blank_image"`
	Manga                      bool   `yaml:"manga"`
	HasCover                   bool   `yaml:"has_cover"`
	LimitMb                    int    `yaml:"limit_mb"`
	StripFirstDirectoryFromToc bool   `yaml:"strip_first_directory_from_toc"`
	SortPathMode               int    `yaml:"sort_path_mode"`
	ForegroundColor            string `yaml:"foreground_color"`
	BackgroundColor            string `yaml:"background_color"`

	// Default Config
	Show  bool `yaml:"-"`
	Save  bool `yaml:"-"`
	Reset bool `yaml:"-"`

	// Other
	Workers    int  `yaml:"-"`
	Dry        bool `yaml:"-"`
	DryVerbose bool `yaml:"-"`
	Quiet      bool `yaml:"-"`
	Version    bool `yaml:"-"`
	Help       bool `yaml:"-"`

	// Internal
	profiles profiles.Profiles
}

// Initialize default options.
func New() *Options {
	return &Options{
		Profile:                    "",
		Quality:                    85,
		Crop:                       true,
		CropRatioLeft:              1,
		CropRatioUp:                1,
		CropRatioRight:             1,
		CropRatioBottom:            3,
		Brightness:                 0,
		Contrast:                   0,
		AutoRotate:                 false,
		AutoSplitDoublePage:        false,
		NoBlankImage:               true,
		Manga:                      false,
		HasCover:                   true,
		LimitMb:                    0,
		StripFirstDirectoryFromToc: false,
		SortPathMode:               1,
		ForegroundColor:            "000",
		BackgroundColor:            "FFF",
		profiles:                   profiles.New(),
	}
}

func (o *Options) Header() string {
	return "Go Comic Converter\n\nOptions:"
}

func (o *Options) String() string {
	var b strings.Builder
	b.WriteString(o.Header())
	for _, v := range []struct {
		K string
		V any
	}{
		{"Input", o.Input},
		{"Output", o.Output},
		{"Author", o.Author},
		{"Title", o.Title},
		{"Workers", o.Workers},
	} {
		b.WriteString(fmt.Sprintf("\n    %-26s: %v", v.K, v.V))
	}
	b.WriteString(o.ShowConfig())
	b.WriteRune('\n')
	return b.String()
}

// Config file: ~/.go-comic-converter.yaml
func (o *Options) FileName() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".go-comic-converter.yaml")
}

// Load config files
func (o *Options) LoadConfig() error {
	f, err := os.Open(o.FileName())
	if err != nil {
		return nil
	}
	defer f.Close()
	err = yaml.NewDecoder(f).Decode(o)
	if err != nil && err.Error() != "EOF" {
		return err
	}

	return nil
}

// Get current settings for fields that can be saved
func (o *Options) ShowConfig() string {
	var profileDesc, viewDesc string
	profile := o.GetProfile()
	if profile != nil {
		profileDesc = fmt.Sprintf(
			"%s - %s - %dx%d",
			o.Profile,
			profile.Description,
			profile.Width,
			profile.Height,
		)

		perfectWidth, perfectHeight := profile.PerfectDim()
		viewDesc = fmt.Sprintf(
			"%dx%d",
			perfectWidth,
			perfectHeight,
		)
	}
	limitmb := "nolimit"
	if o.LimitMb > 0 {
		limitmb = fmt.Sprintf("%d Mb", o.LimitMb)
	}

	sortpathmode := ""
	switch o.SortPathMode {
	case 0:
		sortpathmode = "path=alpha, file=alpha"
	case 1:
		sortpathmode = "path=alphanum, file=alpha"
	case 2:
		sortpathmode = "path=alphanum, file=alphanum"
	}

	var b strings.Builder
	for _, v := range []struct {
		K string
		V any
	}{
		{"Profile", profileDesc},
		{"ViewRatio", fmt.Sprintf("1:%s", strings.TrimRight(fmt.Sprintf("%f", profiles.PerfectRatio), "0"))},
		{"View", viewDesc},
		{"Quality", o.Quality},
		{"Crop", o.Crop},
		{"CropRatio", fmt.Sprintf("%d Left - %d Up - %d Right - %d Bottom", o.CropRatioLeft, o.CropRatioUp, o.CropRatioRight, o.CropRatioBottom)},
		{"Brightness", o.Brightness},
		{"Contrast", o.Contrast},
		{"AutoRotate", o.AutoRotate},
		{"AutoSplitDoublePage", o.AutoSplitDoublePage},
		{"NoBlankImage", o.NoBlankImage},
		{"Manga", o.Manga},
		{"HasCover", o.HasCover},
		{"LimitMb", limitmb},
		{"StripFirstDirectoryFromToc", o.StripFirstDirectoryFromToc},
		{"SortPathMode", sortpathmode},
		{"Foreground Color", fmt.Sprintf("#%s", o.ForegroundColor)},
		{"Background Color", fmt.Sprintf("#%s", o.BackgroundColor)},
	} {
		b.WriteString(fmt.Sprintf("\n    %-26s: %v", v.K, v.V))
	}
	return b.String()
}

// reset all settings to default value
func (o *Options) ResetConfig() error {
	New().SaveConfig()
	return o.LoadConfig()
}

// save all current settings as futur default value
func (o *Options) SaveConfig() error {
	f, err := os.Create(o.FileName())
	if err != nil {
		return err
	}
	defer f.Close()
	return yaml.NewEncoder(f).Encode(o)
}

// shortcut to get current profile
func (o *Options) GetProfile() *profiles.Profile {
	return o.profiles.Get(o.Profile)
}

// all available profiles
func (o *Options) AvailableProfiles() string {
	return o.profiles.String()
}
