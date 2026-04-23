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
  strip_prefix_mode?: 'auto' | 'always' | 'never';
}

export interface RoutingGuidanceRoute {
  path: string;
  service: string;
  audience?: string;
  source?: string;
  websocket?: boolean;
}

export interface ProjectRoutingResponse {
  routing_policy: RoutingPolicy;
  available_services: string[];
  suggested_routes: RoutingGuidanceRoute[];
  effective_public_paths: RoutingGuidanceRoute[];
  warnings: string[];
}

export interface UpdateRoutingPolicyRequest {
  shared_domain?: string;
  routes: RoutingRoute[];
}
