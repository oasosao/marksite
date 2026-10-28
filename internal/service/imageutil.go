package service

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
)

// maxImageDim 图片最大边长（宽或高），超过则等比缩放
const maxImageDim = 1920

// jpegQuality JPEG 压缩质量（1-100）
const jpegQuality = 80

// compressImage 对图片进行优化压缩，返回压缩后的字节。
// 仅处理 JPEG/PNG；GIF/WebP/ICO 等保持原样。
// 尺寸超过 maxImageDim 时强制缩放；未超限时若压缩后更小则替换。
func compressImage(data []byte, ext string) []byte {
	switch ext {
	case ".jpg", ".jpeg":
		return compressJPEG(data)
	case ".png":
		return compressPNG(data)
	default:
		return data
	}
}

// decodeAndResize 解码图片并在需要时等比缩放到 maxImageDim 以内。
// 返回缩放后的图片和是否进行了缩放。
func decodeAndResize(r io.Reader) (image.Image, bool, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, false, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxImageDim && h <= maxImageDim {
		return img, false, nil
	}
	// 等比缩放
	var nw, nh int
	if w > h {
		nw = maxImageDim
		nh = h * maxImageDim / w
	} else {
		nh = maxImageDim
		nw = w * maxImageDim / h
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst, true, nil
}

func compressJPEG(data []byte) []byte {
	img, resized, err := decodeAndResize(bytes.NewReader(data))
	if err != nil {
		return data
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return data
	}
	out := buf.Bytes()
	// 已缩放则直接返回；未缩放时仅在压缩后更小时替换
	if resized || len(out) < len(data) {
		return out
	}
	return data
}

func compressPNG(data []byte) []byte {
	img, resized, err := decodeAndResize(bytes.NewReader(data))
	if err != nil {
		return data
	}
	var buf bytes.Buffer
	// PNG 用最高压缩级别，尽量减小体积
	enc := &png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return data
	}
	out := buf.Bytes()
	// PNG 重编码可能破坏原有调色板/优化特性导致变大，
	// 因此仅在压缩后更小时才替换，否则保留原图
	if len(out) < len(data) {
		return out
	}
	// 若仅缩放未缩小体积，对大尺寸 PNG 仍保留缩放结果（像素更少利于渲染）；
	// 小尺寸 PNG 直接保留原图
	if resized && len(data) > 512<<10 {
		return out
	}
	return data
}

// 确保 gif 包被引用（保留对动画 gif 的原样处理）
var _ = gif.Decode
