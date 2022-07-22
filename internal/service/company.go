package service

import (
	"context"
	"github.com/Thing-repository/backend-server/pkg/core"
	"github.com/sirupsen/logrus"
)

const departmentHeadName = "Head"

//go:generate mockgen -source=auth.go -destination=mock/authMock.go
type userDBTransaction interface {
	PathUser(ctx context.Context, user *core.UserDB) (*core.UserDB, error)
}

type companyDBTransaction interface {
	AddCompany(ctx context.Context, companyBase *core.CompanyBase) (*core.Company, error)
}

type departmentDBTransaction interface {
	AddDepartment(ctx context.Context, departmentBase *core.DepartmentBase) (*core.Department, error)
}

type rightsDBTransaction interface {
	AddCompanyAdmin(ctx context.Context, userId int, companyId int) (*core.CompanyManager, error)
}

type transactionDBTransaction interface {
	InjectTx(ctx context.Context) (context.Context, error)
	CommitTx(ctx context.Context) error
	RollbackTx(ctx context.Context) error
}

type Company struct {
	userDB        userDBTransaction
	companyDB     companyDBTransaction
	departmentDB  departmentDBTransaction
	rightsDB      rightsDBTransaction
	transactionDB transactionDBTransaction
}

func NewCompany(userDB userDBTransaction, companyDB companyDBTransaction,
	departmentDB departmentDBTransaction, rightsDB rightsDBTransaction,
	transactionDB transactionDBTransaction) *Company {
	return &Company{userDB: userDB, companyDB: companyDB,
		departmentDB: departmentDB, rightsDB: rightsDB,
		transactionDB: transactionDB}
}

func (C *Company) AddCompany(companyAdd *core.CompanyBase, user *core.User) (*core.Company, error) {
	logBase := logrus.Fields{
		"module":   "service",
		"function": "AddCompany",
	}

	ctx := context.TODO()

	ctx, err := C.transactionDB.InjectTx(ctx)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":  logBase,
			"error": err.Error(),
		}).Error("error create transaction")
		return nil, err
	}

	companyData, err := C.companyDB.AddCompany(ctx, companyAdd)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":       logBase,
			"companyAdd": companyAdd,
			"error":      err.Error(),
		}).Error("error add company to database")
		return nil, err
	}

	departmentAdd := &core.DepartmentBase{
		DepartmentName: departmentHeadName,
	}

	departmentData, err := C.departmentDB.AddDepartment(ctx, departmentAdd)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":          logBase,
			"departmentAdd": departmentAdd,
			"error":         err.Error(),
		}).Error("error add department to database")
		return nil, err
	}

	_, err = C.rightsDB.AddCompanyAdmin(ctx, user.Id, companyData.Id)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":      logBase,
			"userId":    user.Id,
			"companyId": companyData.Id,
			"error":     err.Error(),
		}).Error("error add company admin to database")
		return nil, err
	}

	newUserDb := &core.UserDB{
		User: core.User{
			CompanyId:    &companyData.Id,
			DepartmentId: &departmentData.Id,
		},
	}

	_, err = C.userDB.PathUser(ctx, newUserDb)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"base":      logBase,
			"newUserDb": newUserDb,
			"error":     err.Error(),
		}).Error("error change user data in database")
		return nil, err
	}

	return companyData, nil
}