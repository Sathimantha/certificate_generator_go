package certificate

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/skip2/go-qrcode"
)

func Generate(name, regNumber, outputDir string) (string, error) {
	// ── Configuration from .env ─────────────────────────────────────────────
	templatePath := os.Getenv("TEMPLATE_IMAGE")
	fontFamily := getEnvOrDefault("FONT_FAMILY", "Helvetica")

	// Get template dimensions in pixels
	templateWidthPx, _ := strconv.ParseFloat(getEnvOrDefault("TEMPLATE_WIDTH_PX", "2500"), 64)
	templateHeightPx, _ := strconv.ParseFloat(getEnvOrDefault("TEMPLATE_HEIGHT_PX", "1932"), 64)

	// Get DPI for conversion
	dpi, _ := strconv.ParseFloat(getEnvOrDefault("DPI", "300"), 64)

	// Calculate page size in mm from pixels and DPI
	pageWidth := (templateWidthPx / dpi) * 25.4
	pageHeight := (templateHeightPx / dpi) * 25.4

	// Ensure landscape orientation
	if pageWidth < pageHeight {
		pageWidth, pageHeight = pageHeight, pageWidth
	}

	// Debug output
	fmt.Printf("Template: %.0fx%.0f px @ %.0f DPI → PDF: %.2fx%.2f mm\n",
		templateWidthPx, templateHeightPx, dpi, pageWidth, pageHeight)

	// ── Text positioning & styling ──────────────────────────────────────────
	nameSize, _ := strconv.ParseFloat(getEnvOrDefault("NAME_SIZE", "42"), 64)
	nameLeft, _ := strconv.ParseFloat(getEnvOrDefault("NAME_LEFT", "50"), 64)
	nameTop, _ := strconv.ParseFloat(getEnvOrDefault("NAME_TOP", "70"), 64)
	nameR, _ := strconv.Atoi(getEnvOrDefault("NAME_COLOR_R", "0"))
	nameG, _ := strconv.Atoi(getEnvOrDefault("NAME_COLOR_G", "0"))
	nameB, _ := strconv.Atoi(getEnvOrDefault("NAME_COLOR_B", "0"))

	regSize, _ := strconv.ParseFloat(getEnvOrDefault("REG_SIZE", "18"), 64)
	regLeft, _ := strconv.ParseFloat(getEnvOrDefault("REG_LEFT", "50"), 64)
	regTop, _ := strconv.ParseFloat(getEnvOrDefault("REG_TOP", "110"), 64)
	regR, _ := strconv.Atoi(getEnvOrDefault("REG_COLOR_R", "0"))
	regG, _ := strconv.Atoi(getEnvOrDefault("REG_COLOR_G", "0"))
	regB, _ := strconv.Atoi(getEnvOrDefault("REG_COLOR_B", "0"))

	qrLeft, _ := strconv.ParseFloat(getEnvOrDefault("QR_LEFT", "160"), 64)
	qrTop, _ := strconv.ParseFloat(getEnvOrDefault("QR_TOP", "110"), 64)
	qrSize, _ := strconv.Atoi(getEnvOrDefault("QR_SIZE", "180"))
	qrLevelStr := getEnvOrDefault("QR_ERROR_CORRECTION", "M")

	// ── Generate QR ─────────────────────────────────────────────────────────
	baseURL := getEnvOrDefault("VERIFICATION_BASE_URL", "https://peaceandhumanity.org/verification")
	baseURL = strings.TrimRight(baseURL, "/")
	verifyURL := fmt.Sprintf("%s#%s", baseURL, regNumber)

	qr, err := qrcode.New(verifyURL, getQRLevel(qrLevelStr))
	if err != nil {
		return "", fmt.Errorf("QR creation failed: %w", err)
	}

	// Get QR as image (this gives us black modules on white bg by default)
	img := qr.Image(qrSize) // qrSize is the pixel size you want

	// Custom colors from .env
	fgR, _ := strconv.Atoi(getEnvOrDefault("QR_FG_R", "0"))
	fgG, _ := strconv.Atoi(getEnvOrDefault("QR_FG_G", "0"))
	fgB, _ := strconv.Atoi(getEnvOrDefault("QR_FG_B", "0"))
	fgA, _ := strconv.Atoi(getEnvOrDefault("QR_FG_A", "255"))

	bgR, _ := strconv.Atoi(getEnvOrDefault("QR_BG_R", "0"))
	bgG, _ := strconv.Atoi(getEnvOrDefault("QR_BG_G", "0"))
	bgB, _ := strconv.Atoi(getEnvOrDefault("QR_BG_B", "0"))
	bgA, _ := strconv.Atoi(getEnvOrDefault("QR_BG_A", "0"))

	// Create new image with desired background (usually transparent)
	customImg := image.NewRGBA(image.Rect(0, 0, qrSize, qrSize))

	// Fill background
	bgColor := color.RGBA{uint8(bgR), uint8(bgG), uint8(bgB), uint8(bgA)}
	draw.Draw(customImg, customImg.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Draw QR modules with custom foreground color
	fgColor := color.RGBA{uint8(fgR), uint8(fgG), uint8(fgB), uint8(fgA)}

	for y := 0; y < qrSize; y++ {
		for x := 0; x < qrSize; x++ {
			if img.At(x, y) == color.Black { // original QR uses black for modules
				customImg.Set(x, y, fgColor)
			}
			// Transparent/white pixels stay as background color
		}
	}

	// Save the custom image
	tempQRPath := filepath.Join(outputDir, "temp_qr_"+regNumber+".png")
	f, err := os.Create(tempQRPath)
	if err != nil {
		return "", fmt.Errorf("cannot create temp QR file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, customImg); err != nil {
		return "", fmt.Errorf("cannot encode custom QR: %w", err)
	}
	defer os.Remove(tempQRPath)

	// ── Create PDF ──────────────────────────────────────────────────────────
	// Keep the working reversed setup (this forces landscape correctly)
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "L",
		UnitStr:        "mm",
		Size: gofpdf.SizeType{
			Wd: pageHeight, // smaller value
			Ht: pageWidth,  // larger value
		},
	})

	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	// Safety buffer to avoid edge clipping (adjust 1.0–3.0 mm based on testing)
	const safety = 1.0

	if templatePath != "" {
		if _, err := os.Stat(templatePath); err == nil {
			pdf.ImageOptions(
				templatePath,
				safety, safety, // shift inward a tiny bit from left/top
				pageWidth-safety*2, pageHeight-safety*2, // shrink very slightly to fit inside safety zone
				false,
				gofpdf.ImageOptions{ImageType: "", ReadDpi: false},
				0, "",
			)
		} else {
			return "", fmt.Errorf("template image not found: %s", templatePath)
		}
	}

	// ── Name (fixed left position - no centering) ───────────────────────────
	pdf.SetFont(fontFamily, "B", nameSize)
	pdf.SetTextColor(nameR, nameG, nameB)
	pdf.SetXY(nameLeft, nameTop)
	pdf.Cell(0, nameSize, name) // 0 = auto width, no forced centering

	// ── Registration Number (fixed left position - no centering) ────────────
	regText := "Registration Number : " + regNumber
	pdf.SetFont(fontFamily, "", regSize)
	pdf.SetTextColor(regR, regG, regB)
	pdf.SetXY(regLeft, regTop)
	pdf.Cell(0, regSize, regText)

	// ── QR Code ─────────────────────────────────────────────────────────────
	qrSizeMM := float64(qrSize) * 25.4 / dpi
	if _, err := os.Stat(tempQRPath); err == nil {
		pdf.ImageOptions(tempQRPath, qrLeft, qrTop, qrSizeMM, qrSizeMM, false,
			gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, 0, "")
	}

	// ── Save PDF ────────────────────────────────────────────────────────────
	filename := sanitize(regNumber + ".pdf")
	outputPath := filepath.Join(outputDir, filename)

	err = pdf.OutputFileAndClose(outputPath)
	if err != nil {
		return "", fmt.Errorf("PDF save failed: %w", err)
	}

	fmt.Printf("PDF generated: %s\n", filename)

	return outputPath, nil
}

func getQRLevel(level string) qrcode.RecoveryLevel {
	switch strings.ToUpper(level) {
	case "L":
		return qrcode.Low
	case "M":
		return qrcode.Medium
	case "Q":
		return qrcode.High
	case "H":
		return qrcode.Highest
	default:
		return qrcode.Medium
	}
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(s))
}
