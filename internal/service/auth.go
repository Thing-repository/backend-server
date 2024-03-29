package service

import (
	"context"
	"github.com/Thing-repository/backend-server/pkg/core"
	"github.com/Thing-repository/backend-server/pkg/core/moduleErrors"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -source=auth.go -destination=mock/authMock.go
type token interface {
	GenerateToken(userId int, credentials []core.Credentials) (string, error)
}

//go:generate mockgen -source=auth.go -destination=mock/authMock.go
type hash interface {
	GenerateHash(password string) (string, error)
	ValidateHash(hash string, password string) error
}

//go:generate mockgen -source=auth.go -destination=mock/authMock.go
type userDB interface {
	GetUserByEmail(ctx context.Context, email string) (*core.UserDB, error)
	GetUser(ctx context.Context, userId int) (*core.UserDB, error)
	AddUser(ctx context.Context, user *core.AddUserDB) (*core.UserDB, error)
}

//go:generate mockgen -source=auth.go -destination=mock/authMock.go
type credentialDBAuth interface {
	GetUserCredential(ctx context.Context, userId int) ([]core.Credentials, error)
}

type transactionDBAuth interface {
	InjectTx(ctx context.Context) (context.Context, error)
	CommitTx(ctx context.Context) error
	RollbackTx(ctx context.Context) error
	RollbackTxDefer(ctx context.Context)
}

type AuthService struct {
	token         token
	db            userDB
	hash          hash
	credentialDB  credentialDBAuth
	transactionDB transactionDBAuth
}

func NewAuth(token token, db userDB, hash hash, credentialDB credentialDBAuth, transactionDB transactionDBCompany) *AuthService {
	return &AuthService{
		token:         token,
		db:            db,
		hash:          hash,
		credentialDB:  credentialDB,
		transactionDB: transactionDB,
	}
}

func (a *AuthService) SignIn(authData *core.UserSignInData) (*core.SignInResponse, error) {
	logBase := logrus.Fields{
		"module":   "service",
		"function": "signIn",
		"email":    authData.UserMail,
	}

	ctx := context.TODO()

	// get user data
	userData, err := a.db.GetUserByEmail(ctx, authData.UserMail)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err,
		}).Error("error get user data")
		switch err {
		case moduleErrors.ErrorDatabaseUserNotFound:
			return nil, moduleErrors.ErrorServiceUserNotFound

		default:
			return nil, err
		}
	}

	// validation password
	err = a.hash.ValidateHash(*userData.PasswordHash, authData.UserPassword)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err,
		}).Error("error validation password")
		switch err {
		case moduleErrors.ErrorHashValidationPassword:
			return nil, moduleErrors.ErrorServiceInvalidPassword
		default:
			return nil, err
		}
	}

	//generate token
	token, err := a.generateTokenForUser(ctx, userData.Id)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err.Error(),
		}).Error("error generate token")
	}

	return &core.SignInResponse{
		User:  userData.User,
		Token: token,
	}, nil
}

func (a *AuthService) SignUp(authData *core.UserSignUpData) (*core.SignInResponse, error) {
	logBase := logrus.Fields{
		"module":   "service",
		"function": "signUp",
	}
	// create struct for add user to database
	userDb := core.AddUserDB{
		UserBaseData: authData.UserBaseData,
	}

	// generate hash for user password
	hash, err := a.hash.GenerateHash(authData.Password)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err,
		}).Error("error generate password hash")
		switch err {
		default:
			return nil, err
		}
	}

	// add hash to data
	userDb.PasswordHash = hash

	ctx := context.TODO()
	ctx, err = a.transactionDB.InjectTx(ctx)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err.Error(),
		}).Error("error create transaction")
		return nil, err
	}

	defer a.transactionDB.RollbackTxDefer(ctx)

	// add user to database
	userData, err := a.db.AddUser(ctx, &userDb)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err,
		}).Error("error add user to database")
		switch err {
		case moduleErrors.ErrorDataBaseUserAlreadyHas:
			return nil, moduleErrors.ErrorServiceUserAlreadyHas
		default:
			return nil, err
		}
	}

	token, err := a.generateTokenForUser(ctx, userData.Id)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err.Error(),
		}).Error("error generate token")
	}

	response := core.SignInResponse{
		User:  userData.User,
		Token: token,
	}

	if err = a.transactionDB.CommitTx(ctx); err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err.Error(),
		}).Error("error commit transaction")
		return nil, err
	}

	return &response, nil
}

func (a *AuthService) generateTokenForUser(ctx context.Context, userId int) (string, error) {
	logBase := logrus.Fields{
		"module":   "service",
		"function": "generateTokenForUser",
		"userId":   userId,
	}
	credentials, err := a.credentialDB.GetUserCredential(ctx, userId)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err.Error(),
		}).Error("error get credentials")
		return "", err
	}

	//generate token
	token, err := a.token.GenerateToken(userId, credentials)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err.Error(),
		}).Error("error generate token")
		switch err {
		default:
			return "", err
		}
	}

	return token, nil
}
