package routes

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"scanner/auth"
	ps "scanner/product_state"
	xutil "scanner/xml_util"
)

var setupPageRoutes = func(p *gin.Engine) {
	p.LoadHTMLGlob("html/*.html")
	p.Static("/sounds", "./sounds")
	p.Static("/images", "./images")

	setupAdminPageRoutes(p.Group("/admin"))

	p.GET("/", func(ctx *gin.Context) {
		ctx.HTML(200, "index.html", nil)
	})

	authorized := p.Group("/")

	authorized.Use(auth.PagesMiddlewareValidateSession(auth.UserRole, auth.UserSessionConst))

	authorized.GET("/products", func(ctx *gin.Context) {
		_, files, err := xutil.DecodeAllXMLToStruct()
		if err != nil {
			ctx.JSON(500, gin.H{
				"status": false,
				"error":  "failed to load files",
			})
			return
		}

		filteredItems := make([]xutil.Item, 0)
		for _, f := range files {
			items := xutil.XmlItemsFromFilename[f]
			newItems := ps.LoadProductState(items, f)
			filteredItems = append(filteredItems, newItems...)
		}

		proto := "ws"
		if ctx.Request.Header.Get("X-Forwarded-Proto") == "https" {
			proto = "wss"
		}

		host := ctx.Request.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = ctx.Request.Host
		}

		ctx.HTML(200, "xml_file.html", gin.H{
			"WebSocketURL": fmt.Sprintf("%s://%s/api/v1/ws", proto, host),
			"Products":     filteredItems,
			"ProductState": ps.GetAllProductStates(),
		})
	})
}

var setupAdminPageRoutes = func(p *gin.RouterGroup) {
	p.GET("/", auth.PagesMiddlewareValidateSession(auth.AdminRole, auth.AdminSessionConst), func(ctx *gin.Context) {
		ctx.HTML(200, "admin_index.html", nil)
	})
}
