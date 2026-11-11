package router

import (
	"fmt"

	"github.com/Massad/gin-boilerplate/controllers"
	"github.com/Massad/gin-boilerplate/forms"
	"github.com/gin-contrib/gzip"
	uuid "github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

//CORSMiddleware ...
//CORS (Cross-Origin Resource Sharing)
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Origin, Authorization, Accept, Client-Security-Token, Accept-Encoding, x-access-token")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			fmt.Println("OPTIONS")
			c.AbortWithStatus(200)
		} else {
			c.Next()
		}
	}
}

//RequestIDMiddleware ...
//Generate a unique ID and attach it to each request for future reference or use
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := uuid.New()
		c.Writer.Header().Set("X-Request-Id", uuid.String())
		c.Next()
	}
}

var auth = new(controllers.AuthController)

//TokenAuthMiddleware ...
//JWT Authentication middleware attached to each request that needs to be authenitcated to validate the access_token in the header
func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth.TokenValid(c)
		c.Next()
	}
}

//New builds the engine with the global middleware and every /v1 route.
func New() (*gin.Engine, error) {
	//Start the default gin server
	r := gin.Default()

	//Custom form validator
	binding.Validator = new(forms.DefaultValidator)

	r.Use(CORSMiddleware())
	r.Use(RequestIDMiddleware())
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	v1 := r.Group("/v1")
	{
		/*** START USER ***/
		user := new(controllers.UserController)

		v1.POST("/login", user.Login)
		v1.POST("/register", user.Register)
		v1.GET("/user/logout", user.Logout)
		v1.PUT("/profile", TokenAuthMiddleware(), user.UpdateProfile)

		/*** START AUTH ***/
		auth := new(controllers.AuthController)

		//Refresh the token when needed to generate new access_token and refresh_token for the user
		v1.POST("/token/refresh", auth.Refresh)
		transaction := new(controllers.TransactionController)
		v1.POST("/topup", TokenAuthMiddleware(), transaction.CreateTopUpTransaction)
		v1.POST("/payment", TokenAuthMiddleware(), transaction.CreatePaymentTransaction)
		v1.POST("/transfer", TokenAuthMiddleware(), transaction.CreateTransferTransaction)
		v1.GET("/transactions", TokenAuthMiddleware(), transaction.All)

		/*** START Article ***/
		// article := new(controllers.ArticleController)

		// v1.POST("/article", TokenAuthMiddleware(), article.Create)
		// v1.GET("/articles", TokenAuthMiddleware(), article.All)
		// v1.GET("/article/:id", TokenAuthMiddleware(), article.One)
		// v1.PUT("/article/:id", TokenAuthMiddleware(), article.Update)
		// v1.DELETE("/article/:id", TokenAuthMiddleware(), article.Delete)
	}

	return r, nil
}
