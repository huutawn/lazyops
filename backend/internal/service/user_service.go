package service

import (
	"errors"

	"lazyops-server/internal/repository"
)

var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	users *repository.UserRepository
}

func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) GetProfile(userID uint) (*UserProfile, error) {
	user, err := s.users.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	profile := ToUserProfile(user)
	return &profile, nil
}
