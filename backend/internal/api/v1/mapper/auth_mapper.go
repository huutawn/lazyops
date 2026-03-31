package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToRegisterCommand(req requestdto.RegisterRequest) service.RegisterCommand {
	return service.RegisterCommand{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	}
}

func ToLoginCommand(req requestdto.LoginRequest) service.LoginCommand {
	return service.LoginCommand{
		Email:    req.Email,
		Password: req.Password,
	}
}

func ToAuthResponse(result service.AuthResult) responsedto.AuthResponse {
	return responsedto.AuthResponse{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		ExpiresIn:   int64(result.ExpiresIn.Seconds()),
		User:        ToUserResponse(result.User),
	}
}

func ToUserResponse(profile service.UserProfile) responsedto.UserResponse {
	return responsedto.UserResponse{
		ID:    profile.ID,
		Name:  profile.Name,
		Email: profile.Email,
		Role:  profile.Role,
	}
}
