package controllers

import (
	"errors"
	"log"
	"net/http"

	"github.com/Massad/gin-boilerplate/forms"
	"github.com/Massad/gin-boilerplate/models"
	"github.com/gin-gonic/gin"
)

//AuthController ...
type AuthController struct{}

var authModel = new(models.AuthModel)

//TokenValid ...
func (ctl AuthController) TokenValid(c *gin.Context) {

	tokenAuth, err := authModel.ExtractTokenMetadata(c.Request)
	if err != nil {
		//Token either expired or not valid
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Please login first"})
		return
	}

	userID, err := authModel.FetchAuth(tokenAuth)
	if err != nil {
		//Token does not exists in Redis (User logged out or expired)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Please login first"})
		return
	}

	//To be called from GetUserID()
	c.Set("userID", userID)
}

//RequireAdmin ...
//Runs after TokenValid. The role is read from the database on each request, so revoking it takes effect immediately.
func (ctl AuthController) RequireAdmin(c *gin.Context) {
	isAdmin, err := userModel.IsAdmin(c.GetString("userID"))
	if err != nil || !isAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Forbidden"})
		return
	}
}

//Refresh ...
func (ctl AuthController) Refresh(c *gin.Context) {
	var tokenForm forms.Token

	if c.ShouldBindJSON(&tokenForm) != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"message": "Invalid form", "form": tokenForm})
		c.Abort()
		return
	}

	//verify the token; if there is an error, it is forged, expired or missing its claims
	refresh, err := authModel.ParseRefreshToken(tokenForm.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid authorization, please login again"})
		return
	}

	//Consume the refresh token and create the next pair of refresh and access tokens
	ts, err := authModel.RotateSession(refresh)
	if errors.Is(err, models.ErrRefreshTokenReused) {
		log.Printf("security: refresh token reused for user %s; session %s revoked", refresh.UserID, refresh.SessionID)
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid authorization, please login again"})
		return
	}

	tokens := map[string]string{
		"access_token":  ts.AccessToken,
		"refresh_token": ts.RefreshToken,
	}
	c.JSON(http.StatusOK, tokens)
}
