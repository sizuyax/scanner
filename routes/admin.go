package routes

import (
	"encoding/json"
	"fmt"
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/gin-gonic/gin"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"scanner/auth"
	"scanner/files"
	ps "scanner/product_state"
	"scanner/users"
	"scanner/websockets"
	xutil "scanner/xml_util"
	"strings"
)

var setupApiAdminRoutes = func(admin *gin.RouterGroup) {

	authorized := admin.Group("/")

	authorized.Use(auth.ApiMiddlewareValidateSession(auth.AdminRole, auth.AdminSessionConst))

	authorized.GET("/logout", func(ctx *gin.Context) {
		ctx.SetCookie(auth.AdminSessionConst, "", -1, "/", "", false, true)

		ctx.JSON(200, gin.H{
			"status":   true,
			"redirect": "/",
			"message":  "logged out successfully",
		})
	})

	filesGroup := authorized.Group("/files")
	{
		filesGroup.POST("/upload", func(ctx *gin.Context) {

			form, err := ctx.MultipartForm()
			if err != nil {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "unable to get file",
				})
				return
			}

			files := form.File["files"]

			if _, err := os.Stat(xutil.XmlFilePath); os.IsNotExist(err) {
				if err := os.MkdirAll(xutil.XmlFilePath, os.ModePerm); err != nil {
					ctx.JSON(500, gin.H{
						"status": false,
						"error":  "unable to create a dir",
					})
					return
				}
			}

			for _, file := range files {

				fileExt := filepath.Ext(file.Filename)
				originalFileName := strings.TrimSuffix(filepath.Base(file.Filename), filepath.Ext(file.Filename))
				filename := strings.ReplaceAll(strings.ToLower(originalFileName), " ", "-") + fileExt

				out, err := os.Create(xutil.XmlFilePath + filename)
				if err != nil {
					ctx.JSON(500, gin.H{
						"status": false,
						"error":  "failed to create file in dir",
					})
					return
				}
				defer out.Close()

				readerFile, _ := file.Open()
				_, err = io.Copy(out, readerFile)
				if err != nil {
					ctx.JSON(500, gin.H{
						"status": false,
						"error":  err.Error(),
					})
					return
				}
			}

			res := websockets.WebSocketResponse{
				Action: "file_uploaded",
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
				"status":  true,
				"message": "xml uploaded successfully",
			})
		})

		filesGroup.GET("/", func(ctx *gin.Context) {
			ctx.JSON(200, gin.H{
				"status": true,
				"files":  files.ContentToStruct("./xml"),
			})
		})

		filesGroup.DELETE("/:filename", func(ctx *gin.Context) {
			filename := ctx.Param("filename")

			if filename == "" {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "filename is required",
				})
				return
			}

			filePath := xutil.XmlFilePath + filename

			ps.RemoveFileState(filename)

			if err := os.Remove(filePath); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to delete file",
				})
				return
			}

			res := websockets.WebSocketResponse{
				Action: "file_deleted",
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
				"status":  true,
				"message": "file deleted successfully",
			})
		})

		filesGroup.GET("/reset_scan_history/:filename", func(ctx *gin.Context) {
			filename := ctx.Param("filename")

			if filename == "" {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "filename is required",
				})
				return
			}

			filePath := xutil.XmlFilePath + filename

			productStates, err := ps.ResetScanHistory(filePath)

			if err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to reset scan history",
				})
				return
			}

			res := gin.H{
				"action": "reset_scan_history",
				"ps":     productStates,
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
				"status":  true,
				"message": "scan history reset successfully",
			})
		})
	}

	reportsGroup := authorized.Group("/reports")
	{
		reportsGroup.GET("/", func(ctx *gin.Context) {
			ctx.JSON(200, gin.H{
				"status":  true,
				"reports": files.ContentToStruct("./xlsx"),
			})
		})

		reportsGroup.GET("/:filename", func(ctx *gin.Context) {
			filename := ctx.Param("filename")

			if filename == "" {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "filename is required",
				})
				return
			}

			reportPath := "./xlsx/" + filename

			if _, err := os.Stat(reportPath); os.IsNotExist(err) {
				ctx.JSON(404, gin.H{
					"status": false,
					"error":  "report not found",
				})
				return
			}

			ctx.File(reportPath)
		})

		reportsGroup.DELETE("/:filename", func(ctx *gin.Context) {
			filename := ctx.Param("filename")

			if filename == "" {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "filename is required",
				})
				return
			}

			reportPath := "./xlsx/" + filename

			if err := os.Remove(reportPath); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to delete file",
				})
				return
			}

			ctx.JSON(200, gin.H{
				"status":  true,
				"message": "report deleted successfully",
			})
		})
	}

	usersGroup := authorized.Group("/users")
	{

		type userRequest struct {
			Username string `form:"username"`
			Role     string `form:"role"`
			Password string `form:"password"`
		}
		usersGroup.POST("/", func(ctx *gin.Context) {
			var req userRequest

			if err := ctx.ShouldBind(&req); err != nil {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "invalid request",
				})
				return
			}

			if err := users.SaveUsers([]users.User{
				{
					Username: req.Username,
					Role:     req.Role,
					Password: req.Password,
				},
			}); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  err.Error(),
				})
				return
			}

			ctx.JSON(200, gin.H{
				"status":  true,
				"message": "user created successfully",
			})
		})

		usersGroup.GET("/", func(ctx *gin.Context) {

			users, err := users.LoadUsers()
			if err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to load users",
				})
				return
			}

			ctx.JSON(200, gin.H{
				"status": true,
				"users":  users,
			})
		})

		usersGroup.DELETE("/:username", func(ctx *gin.Context) {
			username := ctx.Param("username")

			if username == "" {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "username is required",
				})
				return
			}

			if err := users.DeleteUser(username); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to delete user",
				})
				return
			}

			ctx.JSON(200, gin.H{
				"status":  true,
				"message": "user deleted successfully",
			})
		})

		type changePasswordRequest struct {
			Password string `form:"password"`
		}

		usersGroup.POST("/:username/change_password", func(ctx *gin.Context) {
			username := ctx.Param("username")

			if username == "" {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "username is required",
				})
				return
			}

			var req changePasswordRequest

			if err := ctx.ShouldBind(&req); err != nil {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "invalid request",
				})
				return
			}

			err := users.ChangePassword(username, req.Password)
			if err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to change password",
				})
				return
			}

			ctx.JSON(200, gin.H{
				"status":  true,
				"message": "password changed successfully",
			})
		})

		type users struct {
			Username string `json:"username"`
			Role     string `json:"role"`
			Password string `json:"password"`
		}

		usersGroup.GET("/barcode/:username", func(ctx *gin.Context) {

			username := ctx.Param("username")

			if username == "" {
				ctx.JSON(400, gin.H{
					"status": false,
					"error":  "username is required",
				})
				return
			}

			usersFilePath := "json/users.json"

			if _, err := os.Stat(usersFilePath); os.IsNotExist(err) {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "passwords file not found",
				})
				return
			}

			openedFile, err := os.Open(usersFilePath)
			if err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to open file",
				})
				return
			}
			defer openedFile.Close()

			var us []users
			decoder := json.NewDecoder(openedFile)
			if err := decoder.Decode(&us); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to decode file",
				})
				return
			}

			password := ""
			for _, u := range us {
				if u.Username == username {
					password = u.Password
				}
			}

			if password == "" {
				ctx.JSON(404, gin.H{
					"status": false,
					"error":  "password not found for the username",
				})
				return
			}

			barcodeImg, err := code128.Encode(password)
			if err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to generate barcode",
				})
				return
			}

			scaledBarcode, err := barcode.Scale(barcodeImg, 200, 100)
			if err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to scale barcode",
				})
				return
			}

			if err := os.MkdirAll("barcode", os.ModePerm); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to create directory",
				})
				return
			}

			filename := fmt.Sprintf("%s-barcode.png", username)

			filePath := fmt.Sprintf("barcode/%s", filename)
			createdFile, err := os.Create(filePath)
			if err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to create file",
				})
				return
			}
			defer createdFile.Close()

			if err := png.Encode(createdFile, scaledBarcode); err != nil {
				ctx.JSON(500, gin.H{
					"status": false,
					"error":  "failed to encode barcode to PNG",
				})
				return
			}
			defer createdFile.Close()

			ctx.FileAttachment(filePath, filename)
		})
	}
}
