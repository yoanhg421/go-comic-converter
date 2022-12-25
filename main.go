package main

import (
	comicconverter "go-comic-converter/internal/comic-converter"
)

func main() {
	comicconverter.
		New(comicconverter.ComicConverterOptions{
			Quality: 90,
		}).
		Load("/Users/vincent/Downloads/00001.jpg").
		CropMarging().
		Resize(1860, 2480).
		GrayScale().
		Save("/Users/vincent/Downloads/00001_gray.jpg")

}
