package main

import (
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/Massad/gin-boilerplate/models"
	"github.com/Massad/gin-boilerplate/router"
	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
)

func main() {
	//Load the .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("error: failed to load the env file")
	}

	if err := models.ValidateSecrets(); err != nil {
		log.Fatal("error: ", err)
	}
	if _, err := models.LoadTopUpLimits(); err != nil {
		log.Fatal("error: ", err)
	}
	if os.Getenv("PAYMENT_WEBHOOK_SECRET") == "" {
		log.Println("warning: PAYMENT_WEBHOOK_SECRET is not set; POST /v1/webhooks/topup rejects every delivery")
	}

	if os.Getenv("ENV") == "PRODUCTION" {
		gin.SetMode(gin.ReleaseMode)
	}

	r, err := router.New()
	if err != nil {
		log.Fatal("error: ", err)
	}

	//Start PostgreSQL database
	//Example: db.GetDB() - More info in the models folder
	db.Init()

	//Start Redis on database 1 - it's used to store the JWT but you can use it for anythig else
	//Example: db.GetRedis().Set(KEY, VALUE, at.Sub(now)).Err()
	db.InitRedis(1)

	r.LoadHTMLGlob("./public/html/*")

	r.Static("/public", "./public")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"ginBoilerplateVersion": "v0.03",
			"goVersion":             runtime.Version(),
		})
	})

	r.NoRoute(func(c *gin.Context) {
		c.HTML(404, "404.html", gin.H{})
	})

	port := os.Getenv("PORT")

	log.Printf("\n\n PORT: %s \n ENV: %s \n SSL: %s \n Version: %s \n\n", port, os.Getenv("ENV"), os.Getenv("SSL"), os.Getenv("API_VERSION"))

	if os.Getenv("SSL") == "TRUE" {

		//Generated using sh generate-certificate.sh
		SSLKeys := &struct {
			CERT string
			KEY  string
		}{
			CERT: "./cert/myCA.cer",
			KEY:  "./cert/myCA.key",
		}

		r.RunTLS(":"+port, SSLKeys.CERT, SSLKeys.KEY)
	} else {
		r.Run(":" + port)
	}

}
