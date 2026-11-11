package controllers

import (
	"github.com/Massad/gin-boilerplate/forms"
	"github.com/Massad/gin-boilerplate/models"

	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

//UserController ...
type UserController struct{}

var userModel = new(models.UserModel)
var userForm = new(forms.UserForm)
var loginLimiter = models.DefaultLoginLimiter

//getUserID ...
func getUserID(c *gin.Context) (userID string) {
	//MustGet returns the value for the given key if it exists, otherwise it panics.
	return c.MustGet("userID").(string)
}

//Login ...
func (ctrl UserController) Login(c *gin.Context) {
	var loginForm forms.LoginFormDto

	if validationErr := c.ShouldBindJSON(&loginForm); validationErr != nil {
		message := userForm.Login(validationErr)
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": message})
		return
	}

	retryAfter, err := loginLimiter.Reserve(loginForm.PhoneNumber, c.ClientIP())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"message": "Something went wrong, please try again later"})
		return
	}
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(int(retryAfter/time.Second)))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "Too many login attempts, please try again later"})
		return
	}

	user, token, err := userModel.Login(loginForm)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": "Invalid login details"})
		return
	}

	if err := loginLimiter.Succeeded(loginForm.PhoneNumber, c.ClientIP()); err != nil {
		log.Printf("login limiter: could not reset attempts after a successful login: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged in", "user": user.Response(), "token": token})
}

//Register ...
func (ctrl UserController) Register(c *gin.Context) {
	var registerForm forms.RegisterDto

	if validationErr := c.ShouldBindJSON(&registerForm); validationErr != nil {
		message := userForm.Register(validationErr)
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": message})
		return
	}

	user, err := userModel.Register(registerForm)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully registered", "user": user.Response()})
}

//UpdateProfile ...
func (ctrl UserController) UpdateProfile(c *gin.Context) {
	var updateForm forms.UpdateProfileDto
	userID := getUserID(c)

	if validationErr := c.ShouldBindJSON(&updateForm); validationErr != nil {
		message := userForm.Register(validationErr)
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": message})
		return
	}

	user, err := userModel.UpdateProfile(userID, updateForm)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotAcceptable, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated", "user": user.Response()})
}

//Logout ...
func (ctrl UserController) Logout(c *gin.Context) {

	au, err := authModel.ExtractTokenMetadata(c.Request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "User not logged in"})
		return
	}

	if _, err := authModel.FetchAuth(au); err != nil { //already logged out or expired
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid request"})
		return
	}

	if err := authModel.RevokeSession(au.SessionID); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}
