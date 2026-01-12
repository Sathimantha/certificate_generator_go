package certificate

import (
	"fmt"
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
	// Formula: mm = (pixels / DPI) * 25.4
	pageWidth := (templateWidthPx / dpi) * 25.4
	pageHeight := (templateHeightPx / dpi) * 25.4

	// Ensure landscape orientation (width must be greater than height)
	if pageWidth < pageHeight {
		pageWidth, pageHeight = pageHeight, pageWidth
	}

	// Debug output
	fmt.Printf("Template: %.0fx%.0f px @ %.0f DPI → PDF: %.2fx%.2f mm\n",
		templateWidthPx, templateHeightPx, dpi, pageWidth, pageHeight)

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
	verifyURL := fmt.Sprintf("https://peaceandhumanity.org/verification#%s", regNumber)
	qr, err := qrcode.New(verifyURL, getQRLevel(qrLevelStr))
	if err != nil {
		return "", fmt.Errorf("QR creation failed: %w", err)
	}

	tempQRPath := filepath.Join(outputDir, "temp_qr_"+regNumber+".png")
	if err := qr.WriteFile(qrSize, tempQRPath); err != nil {
		return "", fmt.Errorf("QR file write failed: %w", err)
	}
	defer os.Remove(tempQRPath)

	// ── Create PDF with gofpdf (supports absolute positioning) ──────────────
	// gofpdf has a quirk - for landscape, we need to swap Wd and Ht in SizeType
	pdf := gofpdf.New("L", "mm", "", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetMargins(0, 0, 0)

	// Add page with swapped dimensions (gofpdf bug workaround)
	pdf.AddPageFormat("L", gofpdf.SizeType{Wd: pageHeight, Ht: pageWidth})

	// Full-page background image
	if templatePath != "" {
		if _, err := os.Stat(templatePath); err == nil {
			pdf.ImageOptions(templatePath, 0, 0, pageWidth, pageHeight, false,
				gofpdf.ImageOptions{ImageType: "", ReadDpi: false}, 0, "")
		} else {
			return "", fmt.Errorf("template image not found: %s", templatePath)
		}
	}

	// Name overlay with absolute positioning
	pdf.SetFont(fontFamily, "B", nameSize)
	pdf.SetTextColor(nameR, nameG, nameB)
	pdf.SetXY(nameLeft, nameTop)

	// Get the width of the text to center it properly
	nameWidth := pdf.GetStringWidth(name)
	centerX := nameLeft - (nameWidth / 2)
	pdf.SetX(centerX)
	pdf.Cell(nameWidth, nameSize, name)

	// Reg. No overlay
	regText := "Registration Number : " + regNumber
	pdf.SetFont(fontFamily, "", regSize)
	pdf.SetTextColor(regR, regG, regB)
	pdf.SetXY(regLeft, regTop)

	regWidth := pdf.GetStringWidth(regText)
	centerRegX := regLeft - (regWidth / 2)
	pdf.SetX(centerRegX)
	pdf.Cell(regWidth, regSize, regText)

	// QR overlay - convert pixels to mm for display
	qrSizeMM := float64(qrSize) * 25.4 / dpi
	if _, err := os.Stat(tempQRPath); err == nil {
		pdf.ImageOptions(tempQRPath, qrLeft, qrTop, qrSizeMM, qrSizeMM, false,
			gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, 0, "")
	}

	// ── Save PDF ────────────────────────────────────────────────────────────
	filename := sanitize(regNumber + " - " + name + ".pdf")
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
