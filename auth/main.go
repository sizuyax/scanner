package auth

import (
	"github.com/gin-gonic/gin"
)

func PagesMiddlewareValidateSession(role, roleSession string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(roleSession)
		if err != nil || !ValidateSession(role, token) {
			ctx.Redirect(302, "/")
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

func ApiMiddlewareValidateSession(role, roleSession string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, err := ctx.Cookie(roleSession)
		if err != nil || !ValidateSession(role, token) {
			ctx.JSON(401, gin.H{
				"status":   false,
				"code":     10015,
				"error":    "unauthorized",
				"redirect": "/",
			})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
