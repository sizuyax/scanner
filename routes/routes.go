package routes

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"log"
	"os"
	"path/filepath"
	"scanner/auth"
	ps "scanner/product_state"
	"scanner/websockets"
	xutil "scanner/xml_util"
	"strings"
	"time"
)

func SetupAllRoutes(ginDef *gin.Engine) {

	setupPageRoutes(ginDef)

	apiv1 := ginDef.Group("/api/v1")

	setupApiAdminRoutes(apiv1.Group("/admin"))

	type authRequest struct {
		Password string `form:"password"`
	}

	apiv1.POST("/auth", func(ctx *gin.Context) {
		var req authRequest

		if err := ctx.ShouldBind(&req); err != nil {
			ctx.JSON(400, gin.H{
				"status": false,
				"error":  "invalid request",
			})
			return
		}

		username, role, valid := auth.ValidatePassword(req.Password)
		if !valid || role == "" || username == "" {
			ctx.JSON(400, gin.H{
				"status": false,
				"error":  "password mismatch",
			})
			return
		}

		sessionToken := auth.GenerateToken()

		err := auth.SetSession(auth.Session{
			Username:  username,
			Role:      role,
			Token:     sessionToken,
			ExpiresAt: time.Now().Add(auth.SessionDuration),
		})
		if err != nil {
			ctx.JSON(500, gin.H{
				"status": false,
				"error":  "failed to set a session",
			})
			return
		}

		var sessionName string

		isAdmin := false
		if role == auth.AdminRole {
			sessionName = auth.AdminSessionConst
			isAdmin = true
		} else {
			sessionName = auth.UserSessionConst
		}

		ctx.SetCookie(sessionName, sessionToken, int(auth.SessionDuration.Seconds()), "/", "", false, true)

		ctx.JSON(200, gin.H{
			"status":   true,
			"is_admin": isAdmin,
		})
	})

	apiv1.GET("/check_session", func(ctx *gin.Context) {
		token, err := ctx.Cookie(auth.UserSessionConst)
		if err == nil && auth.ValidateSession(auth.UserRole, token) {
			ctx.JSON(200, gin.H{
				"status": true,
			})
			return
		}

		token, err = ctx.Cookie(auth.AdminSessionConst)
		if err == nil && auth.ValidateSession(auth.AdminRole, token) {
			ctx.JSON(200, gin.H{
				"status": true,
			})
			return
		}

		ctx.JSON(401, gin.H{
			"status":   false,
			"code":     10015,
			"error":    "unauthorized",
			"redirect": "/login",
		})
	})

	authorized := apiv1.Group("/")

	authorized.Use(auth.ApiMiddlewareValidateSession(auth.UserRole, auth.UserSessionConst))

	authorized.GET("/logout", func(ctx *gin.Context) {
		ctx.SetCookie(auth.UserSessionConst, "", -1, "/", "", false, true)

		ctx.JSON(200, gin.H{
			"status":   true,
			"redirect": "/login",
			"message":  "logged out successfully",
		})
	})

	// websocket for scanning
	authorized.GET("/ws", websockets.HandleXMLWebSocket)

	authorized.GET("/undo_scan", func(ctx *gin.Context) {
		if len(ps.ScanHistArr) == 0 {
			ctx.JSON(400, gin.H{
				"status": false,
				"error":  "no scans to undo",
				"code":   websockets.NoneScansToUndoCode,
			})
			return
		}

		lastScan := ps.ScanHistArr[len(ps.ScanHistArr)-1]

		state := ps.GetProductState(lastScan.Ean)
		if state == nil {
			ctx.JSON(400, gin.H{
				"status": false,
				"error":  "EAN not found",
				"code":   websockets.EANNotFoundCode,
			})
			return
		}

		state.Scanned -= lastScan.Quantity
		state.Difference = int(state.Scanned - uint(state.Amount))

		ps.SetProductState(lastScan.Ean, state)

		// status = 1 - completed / status = 2 - working / status = 3 - error / status = 0 - waiting
		if state.Difference == 0 {
			state.Status = 1
		} else if state.Difference < 0 && state.Scanned > 0 {
			state.Status = 2
		} else if state.Difference > 0 {
			state.Status = 3
		} else {
			state.Status = 0
		}

		ps.ScanHistArr = ps.ScanHistArr[:len(ps.ScanHistArr)-1]

		res := websockets.WebSocketResponse{
			Action:       "update",
			ProductState: state,
		}

		jsonResponse, err := json.Marshal(res)
		if err != nil {
			ctx.JSON(500, gin.H{
				"status": false,
				"error":  "failed to marshal state",
			})
			return
		}
		websockets.Broadcast(jsonResponse)

		ctx.JSON(200, gin.H{
			"status": true,
			"state":  state,
		})
	})

	authorized.GET("/report", func(ctx *gin.Context) {
		for file, items := range xutil.XmlItemsFromFilename {

			f := excelize.NewFile()

			sheet := "Sheet1"
			f.SetCellValue(sheet, "A1", "Numer Katalogowy")
			f.SetCellValue(sheet, "B1", "Nazwa")
			f.SetCellValue(sheet, "C1", "Ean")
			f.SetCellValue(sheet, "D1", "Ilość")
			f.SetCellValue(sheet, "E1", "Zeskanowano")
			f.SetCellValue(sheet, "F1", "Różnica")
			f.SetCellValue(sheet, "G1", "Błąd")

			for i, p := range items {
				row := i + 2

				var state *ps.ProductState

				if p.Ean != "" {
					state = ps.GetProductState(p.Ean)
				}

				if state == nil {
					state = ps.GetProductState(p.CatalogNumber)
					if state == nil {
						log.Printf("product state not found for CatalogNumber: %s (file: %s)\n", p.CatalogNumber, file)
						continue
					}
				}

				if state.ErrorDesc == ps.ErrDuplicateSameFile.Error() ||
					state.ErrorDesc == ps.ErrDuplicateDiffFiles.Error() {
					if len(state.SourceFiles) > 0 {
						state.ErrorDesc = fmt.Sprintf("%s (First found in: %s)", state.ErrorDesc, state.SourceFiles[0])
					}
				}

				f.SetCellValue(sheet, fmt.Sprintf("A%d", row), p.CatalogNumber)
				f.SetCellValue(sheet, fmt.Sprintf("B%d", row), p.Name)
				f.SetCellValue(sheet, fmt.Sprintf("C%d", row), p.Ean)
				f.SetCellValue(sheet, fmt.Sprintf("D%d", row), p.Amount)
				f.SetCellValue(sheet, fmt.Sprintf("E%d", row), state.Scanned)
				f.SetCellValue(sheet, fmt.Sprintf("F%d", row), state.Difference)
				f.SetCellValue(sheet, fmt.Sprintf("G%d", row), state.ErrorDesc)
			}

			if _, err := os.Stat("./xlsx/"); os.IsNotExist(err) {
				if err := os.MkdirAll("./xlsx/", 0755); err != nil {
					ctx.JSON(500, gin.H{
						"status": false,
						"error":  "failed to create directory",
					})
					return
				}
			}

			filename := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

			finalFilename := fmt.Sprintf("./xlsx/%s-report.xlsx", filename)

			if err := f.SaveAs(finalFilename); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to save Excel file",
				})
				return
			}

			if err := f.Close(); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to close Excel file: " + err.Error(),
				})
				return
			}
		}

		ctx.JSON(200, gin.H{
			"status": true,
		})
	})

}
