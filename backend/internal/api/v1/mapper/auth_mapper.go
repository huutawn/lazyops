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
	}
}

func ToLoginCommand(req requestdto.LoginRequest) service.LoginCommand {
	return service.LoginCommand{
		Email:    req.Email,
		Password: req.Password,
	}
}

func ToCLILoginCommand(req requestdto.CLILoginRequest) service.CLILoginCommand {
	return service.CLILoginCommand{
		AuthFlow:   req.AuthFlow,
		Email:      req.Email,
		Password:   req.Password,
		DeviceName: req.DeviceName,
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

func ToCLILoginResponse(result service.CLIAuthResult) responsedto.CLILoginResponse {
	return responsedto.CLILoginResponse{
		Token:     result.Token,
		TokenType: result.TokenType,
		TokenID:   result.TokenID,
		ExpiresAt: result.ExpiresAt,
		User:      ToUserResponse(result.User),
	}
}

func ToPATRevokeCommand(userID string, req requestdto.PATRevokeRequest) service.RevokePATCommand {
	return service.RevokePATCommand{
		UserID:  userID,
		TokenID: req.TokenID,
	}
}

func ToPATRevokeResponse(result service.PATRevokeResult) responsedto.PATRevokeResponse {
	return responsedto.PATRevokeResponse{
		TokenID: result.TokenID,
		Revoked: result.Revoked,
	}
}

func ToUserResponse(profile service.UserProfile) responsedto.UserResponse {
	return responsedto.UserResponse{
		ID:          profile.ID,
		DisplayName: profile.DisplayName,
		Email:       profile.Email,
		Role:        profile.Role,
		Status:      profile.Status,
	}
}
