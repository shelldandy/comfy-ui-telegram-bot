package image

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
)

// Processor handles image format conversions
type Processor struct {
	jpegQuality int
}

// NewProcessor creates a new image processor
func NewProcessor(jpegQuality int) *Processor {
	return &Processor{
		jpegQuality: jpegQuality,
	}
}

// Result contains both image versions
type Result struct {
	Original       []byte
	Compressed     []byte
	OriginalSize   int
	CompressedSize int
}

// Process takes PNG data and returns both original and compressed versions
func (p *Processor) Process(pngData []byte) (*Result, error) {
	compressed, err := p.CompressToJPEG(pngData)
	if err != nil {
		return nil, err
	}

	return &Result{
		Original:       pngData,
		Compressed:     compressed,
		OriginalSize:   len(pngData),
		CompressedSize: len(compressed),
	}, nil
}

// CompressToJPEG converts PNG bytes to JPEG with configured quality
func (p *Processor) CompressToJPEG(pngData []byte) ([]byte, error) {
	// Decode PNG
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		// Try generic decode in case it's not strictly PNG
		img, _, err = image.Decode(bytes.NewReader(pngData))
		if err != nil {
			return nil, fmt.Errorf("decode image: %w", err)
		}
	}

	// Encode as JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: p.jpegQuality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), nil
}
