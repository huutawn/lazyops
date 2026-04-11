export interface RoutingPolicy {
  shared_domain?: string;
  routes: RoutingRoute[];
}

export interface RoutingRoute {
  path: string;
  service: string;
  port?: number;
  websocket?: boolean;
  strip_prefix?: boolean;
}

export interface ProjectRoutingResponse {
  routing_policy: RoutingPolicy;
  available_services: string[];
}

export interface UpdateRoutingPolicyRequest {
  shared_domain?: string;
  routes: RoutingRoute[];
}
