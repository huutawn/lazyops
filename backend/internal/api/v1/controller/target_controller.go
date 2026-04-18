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
	clusterNodes *service.ClusterNodeService
	bootstrap    *service.BootstrapOrchestrator
}

func NewTargetController(meshNetworks *service.MeshNetworkService, clusters *service.ClusterService) *TargetController {
	return &TargetController{
		meshNetworks: meshNetworks,
		clusters:     clusters,
	}
}

func (ctl *TargetController) WithBootstrapOrchestrator(bootstrap *service.BootstrapOrchestrator) *TargetController {
	ctl.bootstrap = bootstrap
	return ctl
}

func (ctl *TargetController) WithClusterNodeService(clusterNodes *service.ClusterNodeService) *TargetController {
	ctl.clusterNodes = clusterNodes
	return ctl
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

	if ctl.bootstrap != nil {
		_ = ctl.bootstrap.OnInventoryChanged(claims.UserID)
	}
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

	if ctl.bootstrap != nil {
		_ = ctl.bootstrap.OnInventoryChanged(claims.UserID)
	}
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

func (ctl *TargetController) ListClusterNodes(c *gin.Context) {
	if ctl.clusterNodes == nil {
		response.JSON(c, http.StatusOK, "cluster nodes loaded", mapper.ToClusterNodeListResponse(service.ClusterNodeListResult{Items: []service.ClusterNodeRecord{}}))
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.clusterNodes.ListClusterNodes(claims.UserID, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load cluster nodes", "invalid_input", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "failed to load cluster nodes", "cluster_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load cluster nodes", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "cluster nodes loaded", mapper.ToClusterNodeListResponse(*result))
}

func (ctl *TargetController) ConnectClusterNodeSSH(c *gin.Context) {
	if ctl.clusterNodes == nil {
		response.Error(c, http.StatusNotImplemented, "cluster node join is not enabled", "not_enabled", nil)
		return
	}

	var req requestdto.ConnectClusterNodeSSHRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.clusterNodes.ConnectNodeViaSSH(c.Request.Context(), mapper.ToConnectClusterNodeSSHCommand(claims.UserID, c.Param("id"), req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "cluster node join failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrSSHAuthenticationRequired):
			response.Error(c, http.StatusBadRequest, "cluster node join failed", "ssh_auth_required", err.Error())
		case errors.Is(err, service.ErrSSHConnectionFailed):
			response.Error(c, http.StatusBadGateway, "cluster node join failed", "ssh_connection_failed", err.Error())
		case errors.Is(err, service.ErrSSHExecutionFailed):
			response.Error(c, http.StatusBadGateway, "cluster node join failed", "ssh_execution_failed", err.Error())
		case errors.Is(err, service.ErrK3sBootstrapIncomplete):
			response.Error(c, http.StatusBadGateway, "cluster node join failed", "k3s_join_incomplete", err.Error())
		case errors.Is(err, service.ErrTargetNotFound):
			response.Error(c, http.StatusNotFound, "cluster node join failed", "cluster_not_found", err.Error())
		case errors.Is(err, service.ErrClusterNotReady):
			response.Error(c, http.StatusConflict, "cluster node join failed", "cluster_not_ready", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "cluster node join failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "cluster node joined via ssh", mapper.ToConnectClusterNodeSSHResponse(*result))
}
