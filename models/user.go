package models

import (
	"errors"

	"github.com/Massad/gin-boilerplate/db"
	"github.com/Massad/gin-boilerplate/forms"
"github.com/google/uuid" // Import the UUID package
	"golang.org/x/crypto/bcrypt"
)

//User ...
// type User struct {
// 	ID        int64  `db:"id, primarykey, autoincrement" json:"id"`
// 	Email     string `db:"email" json:"email"`
// 	Password  string `db:"password" json:"-"`
// 	Name      string `db:"name" json:"name"`
// 	UpdatedAt int64  `db:"updated_at" json:"-"`
// 	CreatedAt int64  `db:"created_at" json:"-"`
// 	PIN string `db:"email" json:"email"`
// }
type User struct {
	ID          string `db:"id, primarykey" json:"id"` // Changed to string for UUID
	PhoneNumber     string `db:"phone_number" json:"phone_number"`
	FirstName  string `db:"first_name" json:"first_name"`
	LastName      string `db:"last_name" json:"last_name"`
	Address      string `db:"address" json:"address"`
	UpdatedAt int64  `db:"updated_at" json:"-"`
	CreatedAt int64  `db:"created_at" json:"-"`
	PIN string `db:"pin" json:"PIN"`
}

//UserModel ...
type UserModel struct{}

var authModel = new(AuthModel)

//Login ...
// func (m UserModel) Login(form forms.LoginForm) (user User, token Token, err error) {

// 	err = db.GetDB().SelectOne(&user, "SELECT id, email, password, name, updated_at, created_at FROM public.user WHERE email=LOWER($1) LIMIT 1", form.Email)

// 	if err != nil {
// 		return user, token, err
// 	}

// 	//Compare the password form and database if match
// 	bytePassword := []byte(form.Password)
// 	byteHashedPassword := []byte(user.Password)

// 	err = bcrypt.CompareHashAndPassword(byteHashedPassword, bytePassword)

// 	if err != nil {
// 		return user, token, err
// 	}

// 	//Generate the JWT auth token
// 	tokenDetails, err := authModel.CreateToken(user.ID)
// 	if err != nil {
// 		return user, token, err
// 	}

// 	saveErr := authModel.CreateAuth(user.ID, tokenDetails)
// 	if saveErr == nil {
// 		token.AccessToken = tokenDetails.AccessToken
// 		token.RefreshToken = tokenDetails.RefreshToken
// 	}

// 	return user, token, nil
// }
//Login ...
func (m UserModel) Login(form forms.LoginFormDto) (user User, token Token, err error) {

	err = db.GetDB().SelectOne(&user, "SELECT id, password FROM public.user WHERE phone_number=LOWER($1) LIMIT 1", form.PhoneNumber)

	if err != nil {
		return user, token, err
	}

	//Compare the password form and database if match
	bytePassword := []byte(form.PIN)
	byteHashedPassword := []byte(user.PIN)

	err = bcrypt.CompareHashAndPassword(byteHashedPassword, bytePassword)

	if err != nil {
		return user, token, err
	}

	//Generate the JWT auth token
	tokenDetails, err := authModel.CreateToken(user.ID)
	if err != nil {
		return user, token, err
	}

	saveErr := authModel.CreateAuth(user.ID, tokenDetails)
	if saveErr == nil {
		token.AccessToken = tokenDetails.AccessToken
		token.RefreshToken = tokenDetails.RefreshToken
	}

	return user, token, nil
}
//Register ...
// func (m UserModel) Register(form forms.RegisterForm) (user User, err error) {
// 	getDb := db.GetDB()

// 	//Check if the user exists in database
// 	checkUser, err := getDb.SelectInt("SELECT count(id) FROM public.user WHERE email=LOWER($1) LIMIT 1", form.Email)
// 	if err != nil {
// 		return user, errors.New("something went wrong, please try again later")
// 	}

// 	if checkUser > 0 {
// 		return user, errors.New("email already exists")
// 	}

// 	bytePassword := []byte(form.Password)
// 	hashedPassword, err := bcrypt.GenerateFromPassword(bytePassword, bcrypt.DefaultCost)
// 	if err != nil {
// 		return user, errors.New("something went wrong, please try again later")
// 	}

// 	//Create the user and return back the user ID
// 	err = getDb.QueryRow("INSERT INTO public.user(email, password, name) VALUES($1, $2, $3) RETURNING id", form.Email, string(hashedPassword), form.Name).Scan(&user.ID)
// 	if err != nil {
// 		return user, errors.New("something went wrong, please try again later")
// 	}

// 	user.Name = form.Name
// 	user.Email = form.Email

// 	return user, err
// }
//Register ...
func (m UserModel) Register(form forms.RegisterDto) (user User, err error) {
	getDb := db.GetDB()

	//Check if the user exists in database
	checkUser, err := getDb.SelectInt("SELECT count(id) FROM public.user WHERE phone_number=LOWER($1) LIMIT 1", form.PhoneNumber)
	if err != nil {
		return user, errors.New("something went wrong, please try again later")
	}

	if checkUser > 0 {
		return user, errors.New("Phone Number already registered")
	}

	// Generate a UUID for the user ID
	userID := uuid.New().String()

	bytePassword := []byte(form.PIN)
	hashedPassword, err := bcrypt.GenerateFromPassword(bytePassword, bcrypt.DefaultCost)
	if err != nil {
		return user, errors.New("something went wrong, please try again later")
	}

	//Create the user and return back the user ID
	err = getDb.QueryRow("INSERT INTO public.user(id, first_name, last_name, phone_number, address, pin) VALUES($1, $2, $3, $4, $5, $6) RETURNING id",
		userID, form.FirstName, form.LastName, form.PhoneNumber, form.Address, hashedPassword).Scan(&user.ID)
	if err != nil {
		return user, errors.New("something went wrong, please try again later")
	}
	user.ID = userID
	user.FirstName = form.FirstName
	user.LastName = form.LastName
	user.PhoneNumber = form.PhoneNumber
	user.Address = form.Address

	return user, err
}
//One ...
func (m UserModel) One(userID int64) (user User, err error) {
	err = db.GetDB().SelectOne(&user, "SELECT id, phone_number, first_name FROM public.user WHERE id=$1 LIMIT 1", userID)
	return user, err
}
