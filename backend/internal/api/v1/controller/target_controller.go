package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type TargetController struct {
	meshNetworks *service.MeshNetworkService
	clusters     *service.ClusterService
}

func NewTargetController(meshNetworks *service.MeshNetworkService, clusters *service.ClusterService) *TargetController {
	return &TargetController{
		meshNetworks: meshNetworks,
		clusters:     clusters,
	}
}

func (ctl *TargetController) CreateMeshNetwork(c *gin.Context) {
	var req requestdto.CreateMeshNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.meshNetworks.Create(mapper.ToCreateMeshNetworkCommand(claims.UserID, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidProvider):
			response.Error(c, http.StatusBadRequest, "mesh network creation failed", "invalid_provider", err.Error())
		case errors.Is(err, service.ErrInvalidCIDR):
			response.Error(c, http.StatusBadRequest, "mesh network creation failed", "invalid_cidr", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "mesh network creation failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrMeshNetworkNameExists):
			response.Error(c, http.StatusConflict, "mesh network creation failed", "mesh_network_name_exists", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "mesh network creation failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "mesh network created", mapper.ToMeshNetworkSummaryResponse(*result))
}

func (ctl *TargetController) ListMeshNetworks(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.meshNetworks.List(claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "failed to load mesh networks", "invalid_input", err.Error())
			return
		}

		response.Error(c, http.StatusInternalServerError, "failed to load mesh networks", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "mesh networks loaded", mapper.ToMeshNetworkListResponse(*result))
}

func (ctl *TargetController) CreateCluster(c *gin.Context) {
	var req requestdto.CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.clusters.Create(mapper.ToCreateClusterCommand(claims.UserID, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidProvider):
			response.Error(c, http.StatusBadRequest, "cluster creation failed", "invalid_provider", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "cluster creation failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrClusterNameExists):
			response.Error(c, http.StatusConflict, "cluster creation failed", "cluster_name_exists", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "cluster creation failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "cluster created", mapper.ToClusterSummaryResponse(*result))
}

func (ctl *TargetController) ListClusters(c *gin.Context) {
	claims := middleware.MustClaims(c)
	result, err := ctl.clusters.List(claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, "failed to load clusters", "invalid_input", err.Error())
			return
		}

		response.Error(c, http.StatusInternalServerError, "failed to load clusters", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "clusters loaded", mapper.ToClusterListResponse(*result))
}
