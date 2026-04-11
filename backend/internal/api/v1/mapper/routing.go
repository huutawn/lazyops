package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToProjectRoutingResponse(result service.ProjectRoutingResult) responsedto.ProjectRoutingResponse {
	routes := make([]responsedto.RoutingRouteResponse, 0, len(result.RoutingPolicy.Routes))
	for _, r := range result.RoutingPolicy.Routes {
		routes = append(routes, responsedto.RoutingRouteResponse{
			Path:        r.Path,
			Service:     r.Service,
			Port:        r.Port,
			WebSocket:   r.WebSocket,
			StripPrefix: r.StripPrefix,
		})
	}

	return responsedto.ProjectRoutingResponse{
		RoutingPolicy: responsedto.RoutingPolicyResponse{
			SharedDomain: result.RoutingPolicy.SharedDomain,
			Routes:       routes,
		},
		AvailableServices: result.AvailableServices,
	}
}

func ToUpdateRoutingCommand(userID, role, projectID string, req requestdto.UpdateRoutingPolicyRequest) service.UpdateRoutingCommand {
	routes := make([]service.RoutingRouteRecord, 0, len(req.Routes))
	for _, r := range req.Routes {
		routes = append(routes, service.RoutingRouteRecord{
			Path:        r.Path,
			Service:     r.Service,
			Port:        r.Port,
			WebSocket:   r.WebSocket,
			StripPrefix: r.StripPrefix,
		})
	}

	return service.UpdateRoutingCommand{
		UserID:       userID,
		Role:         role,
		ProjectID:    projectID,
		SharedDomain: req.SharedDomain,
		Routes:       routes,
	}
}
