package repository

import (
	"errors"
	"gorm.io/gorm"
	"lazyops-server/internal/models"
)
type UserRepository struct{
	db *gorm.DB
}
func NewUserRepository(db *gorm.DB)*UserRepository{
	return &UserRepository{db:db}
}
func (r *UserRepository) Create(user *models.User) error{
	return r.db.Create(user).Error
}
func (r *UserRepository) GetByEmail(email string)(*models.User,error){
	var user models.User
	if err:= r.db.Where("email = ?",email).First(&user).Error; err != nil{
		if errors.Is(err,gorm.ErrRecordNotFound){
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
func (r *UserRepository) GetByID(id uint)(*models.User,error){
	var user models.User
	if err:= r.db.First(&user,id).Error;err!=nil{
		if errors.Is(err,gorm.ErrRecordNotFound){
			return nil, nil
		}
		return nil, err

	}
	return &user, nil
}
