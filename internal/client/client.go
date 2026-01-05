/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package client

import (
	"google.golang.org/grpc"

	fulfillmentv1 "github.com/innabox/fulfillment-common/api/fulfillment/v1"
)

// ProviderData holds the gRPC clients that are passed to resources and data sources.
type ProviderData struct {
	Conn                           *grpc.ClientConn
	ClustersClient                 fulfillmentv1.ClustersClient
	ClusterTemplatesClient         fulfillmentv1.ClusterTemplatesClient
	ComputeInstancesClient         fulfillmentv1.ComputeInstancesClient
	ComputeInstanceTemplatesClient fulfillmentv1.ComputeInstanceTemplatesClient
	HostsClient                    fulfillmentv1.HostsClient
	HostClassesClient              fulfillmentv1.HostClassesClient
	HostPoolsClient                fulfillmentv1.HostPoolsClient
}
