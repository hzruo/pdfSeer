package pdf

/*
#cgo pkg-config: vips
#include "vips/vips.h"
#include <stdlib.h>

// 自定义 PDF 加载函数，支持页面参数
int vips_pdfload_buffer_page(void *buf, size_t len, VipsImage **out, int page) {
    return vips_pdfload_buffer(buf, len, out, "page", page, "access", VIPS_ACCESS_RANDOM, NULL);
}

// 将 VipsImage 转换为 JPEG 数据
int vips_image_to_jpeg(VipsImage *in, void **buf, size_t *len, int quality) {
    return vips_jpegsave_buffer(in, buf, len, "Q", quality, "strip", 1, NULL);
}
*/
import "C"
import (
	"fmt"
	"io/ioutil"
	"unsafe"
)

// PageRenderResult 页面渲染结果
type PageRenderResult struct {
	ImageData []byte
	Width     int
	Height    int
}

// renderPDFPageWithVips 使用原生 libvips 渲染 PDF 页面
func (p *PDFProcessor) renderPDFPageWithVips(pdfPath string, pageNum int) (*PageRenderResult, error) {
	fmt.Printf("[DEBUG] 使用原生 libvips 渲染第%d页，PDF文件: %s\n", pageNum, pdfPath)

	// 读取 PDF 文件
	pdfData, err := ioutil.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("读取PDF文件失败: %w", err)
	}

	fmt.Printf("[DEBUG] PDF文件大小: %d bytes，页面: %d\n", len(pdfData), pageNum)

	// 准备 C 函数参数
	var image *C.VipsImage
	buf := unsafe.Pointer(&pdfData[0])
	length := C.size_t(len(pdfData))
	page := C.int(pageNum - 1) // libvips 页面索引从0开始

	// 调用自定义的 PDF 加载函数
	err_code := C.vips_pdfload_buffer_page(buf, length, &image, page)
	if err_code != 0 {
		return nil, fmt.Errorf("libvips PDF 加载失败，错误代码: %d", err_code)
	}
	defer C.g_object_unref(C.gpointer(image))

	width := int(image.Xsize)
	height := int(image.Ysize)
	fmt.Printf("[DEBUG] 成功加载第%d页，图片尺寸: %dx%d\n", pageNum, width, height)

	// 转换为 JPEG 数据
	var jpegBuf unsafe.Pointer
	var jpegLen C.size_t
	quality := C.int(90)

	err_code = C.vips_image_to_jpeg(image, &jpegBuf, &jpegLen, quality)
	if err_code != 0 {
		return nil, fmt.Errorf("转换为JPEG失败，错误代码: %d", err_code)
	}
	defer C.g_free(C.gpointer(jpegBuf))

	// 将 C 数据转换为 Go 字节数组
	imageData := C.GoBytes(jpegBuf, C.int(jpegLen))
	fmt.Printf("[DEBUG] 成功转换为JPEG，数据大小: %d bytes\n", len(imageData))

	return &PageRenderResult{
		ImageData: imageData,
		Width:     width,
		Height:    height,
	}, nil
}
