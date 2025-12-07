package instance

import (
	"context"
	"fmt"
	"net"
	"sort"

	configv1alpha1 "github.com/six-group/haproxy-operator/apis/config/v1alpha1"
	proxyv1alpha1 "github.com/six-group/haproxy-operator/apis/proxy/v1alpha1"
	"github.com/six-group/haproxy-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) reconcileService(ctx context.Context, instance *proxyv1alpha1.Instance, listens *configv1alpha1.ListenList, frontends *configv1alpha1.FrontendList) error {
	logger := log.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.GetServiceName(instance),
			Namespace: instance.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := controllerutil.SetOwnerReference(instance, service, r.Scheme); err != nil {
			return err
		}

		svcSpec := instance.Spec.Network.Service

		// Type
		if svcSpec.Type != nil {
			service.Spec.Type = *svcSpec.Type
		}

		// Labels - merge app selector labels with custom labels
		service.Labels = utils.GetAppSelectorLabels(instance)
		for k, v := range svcSpec.Labels {
			service.Labels[k] = v
		}

		// Annotations
		if svcSpec.Annotations != nil {
			service.Annotations = svcSpec.Annotations
		}

		// ClusterIP
		if svcSpec.ClusterIP != "" {
			service.Spec.ClusterIP = svcSpec.ClusterIP
		}

		// LoadBalancer configuration
		if svcSpec.LoadBalancerIP != "" {
			service.Spec.LoadBalancerIP = svcSpec.LoadBalancerIP
		}
		if svcSpec.LoadBalancerClass != nil {
			service.Spec.LoadBalancerClass = svcSpec.LoadBalancerClass
		}
		if len(svcSpec.LoadBalancerSourceRanges) > 0 {
			service.Spec.LoadBalancerSourceRanges = svcSpec.LoadBalancerSourceRanges
		}
		if svcSpec.AllocateLoadBalancerNodePorts != nil {
			service.Spec.AllocateLoadBalancerNodePorts = svcSpec.AllocateLoadBalancerNodePorts
		}

		// Traffic Policies
		if svcSpec.ExternalTrafficPolicy != "" {
			service.Spec.ExternalTrafficPolicy = svcSpec.ExternalTrafficPolicy
		}
		if svcSpec.InternalTrafficPolicy != nil {
			service.Spec.InternalTrafficPolicy = svcSpec.InternalTrafficPolicy
		}
		if svcSpec.HealthCheckNodePort > 0 {
			service.Spec.HealthCheckNodePort = svcSpec.HealthCheckNodePort
		}

		// Session Affinity
		if svcSpec.SessionAffinity != "" {
			service.Spec.SessionAffinity = svcSpec.SessionAffinity
		}
		if svcSpec.SessionAffinityConfig != nil {
			service.Spec.SessionAffinityConfig = svcSpec.SessionAffinityConfig
		}

		// IP Families (Dual-Stack)
		if svcSpec.IPFamilyPolicy != nil {
			service.Spec.IPFamilyPolicy = svcSpec.IPFamilyPolicy
		}
		if len(svcSpec.IPFamilies) > 0 {
			service.Spec.IPFamilies = svcSpec.IPFamilies
		}

		if len(instance.Spec.Network.HostIPs) == 0 {
			service.Spec.Selector = utils.GetAppSelectorLabels(instance)
		}

		service.Spec.Ports = []corev1.ServicePort{}
		for _, listen := range listens.Items {
			for _, bind := range listen.Spec.Binds {
				if ptr.Deref(bind.Hidden, false) {
					continue
				}

				service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
					Name:       bind.Name,
					Port:       bind.Port,
					TargetPort: intstr.FromInt32(bind.Port),
					Protocol:   corev1.ProtocolTCP,
				})
			}
		}

		for _, frontend := range frontends.Items {
			for _, bind := range frontend.Spec.Binds {
				if ptr.Deref(bind.Hidden, false) {
					continue
				}

				service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
					Name:       bind.Name,
					Port:       bind.Port,
					TargetPort: intstr.FromInt32(bind.Port),
					Protocol:   corev1.ProtocolTCP,
				})
			}
		}

		if instance.Spec.Metrics != nil && instance.Spec.Metrics.Enabled {
			service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
				Name:       "metrics",
				Port:       instance.Spec.Metrics.Port,
				TargetPort: intstr.FromInt32(instance.Spec.Metrics.Port),
				Protocol:   corev1.ProtocolTCP,
			})
		}

		service.Spec.Ports = removeDuplicatesByPort(service.Spec.Ports)

		// Apply NodePorts if specified
		if len(svcSpec.NodePorts) > 0 {
			applyNodePorts(service, svcSpec.NodePorts)
		}

		sort.Slice(service.Spec.Ports, func(i, j int) bool {
			return service.Spec.Ports[i].Name < service.Spec.Ports[j].Name
		})

		return nil
	})
	if err != nil {
		return err
	}
	if result != controllerutil.OperationResultNone {
		logger.Info(fmt.Sprintf("Object %s", result), "service", service.Name)
	}

	if len(instance.Spec.Network.HostIPs) > 0 {
		if err := r.reconcileServiceEndpoints(ctx, instance, service); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) reconcileServiceEndpoints(ctx context.Context, instance *proxyv1alpha1.Instance, service *corev1.Service) error {
	logger := log.FromContext(ctx)

	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.GetServiceName(instance),
			Namespace: instance.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, endpointSlice, func() error {
		if err := controllerutil.SetOwnerReference(instance, endpointSlice, r.Scheme); err != nil {
			return err
		}

		var addresses []discoveryv1.Endpoint
		for host, ip := range instance.Spec.Network.HostIPs {
			addresses = append(addresses, discoveryv1.Endpoint{
				Addresses: []string{ip},
				NodeName:  ptr.To(host),
			})
		}
		sort.Slice(addresses, func(i, j int) bool {
			return addresses[i].Addresses[0] < addresses[j].Addresses[0]
		})

		var ports []discoveryv1.EndpointPort
		for _, port := range service.Spec.Ports {
			ports = append(ports, discoveryv1.EndpointPort{
				Name:     ptr.To(port.Name),
				Port:     ptr.To(port.Port),
				Protocol: ptr.To(port.Protocol),
			})
		}
		sort.Slice(ports, func(i, j int) bool {
			return *ports[i].Name < *ports[j].Name
		})

		endpointSlice.Endpoints = addresses
		endpointSlice.Ports = ports
		ipType := detectIPType(addresses[0].Addresses[0])
		endpointSlice.AddressType = ipType

		return nil
	})
	if err != nil {
		return err
	}
	if result != controllerutil.OperationResultNone {
		logger.Info(fmt.Sprintf("Object %s", result), "endpoints", service.Name)
	}

	return nil
}

func removeDuplicatesByPort(ports []corev1.ServicePort) []corev1.ServicePort {
	seen := make(map[int32]bool)
	var result []corev1.ServicePort
	for _, item := range ports {
		if !seen[item.Port] {
			seen[item.Port] = true
			result = append(result, item)
		}
	}
	return result
}

func detectIPType(address string) discoveryv1.AddressType {
	ip := net.ParseIP(address)
	if ip.To4() != nil {
		return discoveryv1.AddressTypeIPv4
	}
	return discoveryv1.AddressTypeIPv6
}

func applyNodePorts(service *corev1.Service, nodePorts []proxyv1alpha1.NodePortSpec) {
	nodePortMap := make(map[string]int32)
	for _, np := range nodePorts {
		nodePortMap[np.Name] = np.NodePort
	}

	for i, port := range service.Spec.Ports {
		if nodePort, ok := nodePortMap[port.Name]; ok {
			service.Spec.Ports[i].NodePort = nodePort
		}
	}
}
