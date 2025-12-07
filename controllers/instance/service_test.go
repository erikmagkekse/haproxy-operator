package instance

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1alpha1 "github.com/six-group/haproxy-operator/apis/config/v1alpha1"
	proxyv1alpha1 "github.com/six-group/haproxy-operator/apis/proxy/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Service Reconcile", Label("controller"), func() {
	Context("ServiceSpec Configuration", func() {
		var (
			scheme   *runtime.Scheme
			ctx      context.Context
			proxy    *proxyv1alpha1.Instance
			initObjs []client.Object
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(scheme)).ShouldNot(HaveOccurred())
			Expect(configv1alpha1.AddToScheme(scheme)).ShouldNot(HaveOccurred())
			Expect(proxyv1alpha1.AddToScheme(scheme)).ShouldNot(HaveOccurred())

			ctx = context.Background()

			proxy = &proxyv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "default",
					UID:       uuid.NewUUID(),
				},
				Spec: proxyv1alpha1.InstanceSpec{
					Configuration: proxyv1alpha1.Configuration{
						Global:   proxyv1alpha1.GlobalConfiguration{},
						Defaults: proxyv1alpha1.DefaultsConfiguration{},
					},
					Network: proxyv1alpha1.Network{
						Service: proxyv1alpha1.ServiceSpec{
							Enabled: true,
						},
					},
				},
			}

			initObjs = []client.Object{proxy}
		})

		It("creates service with LoadBalancer type", func() {
			lbType := corev1.ServiceTypeLoadBalancer
			proxy.Spec.Network.Service.Type = &lbType
			proxy.Spec.Network.Service.LoadBalancerIP = "10.0.0.100"

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(svc.Spec.LoadBalancerIP).Should(Equal("10.0.0.100"))
		})

		It("creates service with ExternalTrafficPolicy Local", func() {
			lbType := corev1.ServiceTypeLoadBalancer
			proxy.Spec.Network.Service.Type = &lbType
			proxy.Spec.Network.Service.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Spec.ExternalTrafficPolicy).Should(Equal(corev1.ServiceExternalTrafficPolicyLocal))
		})

		It("creates service with SessionAffinity ClientIP", func() {
			timeout := int32(10800)
			proxy.Spec.Network.Service.SessionAffinity = corev1.ServiceAffinityClientIP
			proxy.Spec.Network.Service.SessionAffinityConfig = &corev1.SessionAffinityConfig{
				ClientIP: &corev1.ClientIPConfig{
					TimeoutSeconds: &timeout,
				},
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Spec.SessionAffinity).Should(Equal(corev1.ServiceAffinityClientIP))
			Expect(*svc.Spec.SessionAffinityConfig.ClientIP.TimeoutSeconds).Should(Equal(timeout))
		})

		It("creates service with Dual-Stack configuration", func() {
			policy := corev1.IPFamilyPolicyPreferDualStack
			proxy.Spec.Network.Service.IPFamilyPolicy = &policy
			proxy.Spec.Network.Service.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(*svc.Spec.IPFamilyPolicy).Should(Equal(policy))
			Expect(svc.Spec.IPFamilies).Should(ContainElement(corev1.IPv4Protocol))
			Expect(svc.Spec.IPFamilies).Should(ContainElement(corev1.IPv6Protocol))
		})

		It("creates service with custom annotations and labels", func() {
			proxy.Spec.Network.Service.Annotations = map[string]string{
				"service.beta.kubernetes.io/azure-load-balancer-internal": "true",
			}
			proxy.Spec.Network.Service.Labels = map[string]string{
				"environment": "production",
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Annotations["service.beta.kubernetes.io/azure-load-balancer-internal"]).Should(Equal("true"))
			Expect(svc.Labels["environment"]).Should(Equal("production"))
		})

		It("creates service with NodePort configuration", func() {
			npType := corev1.ServiceTypeNodePort
			proxy.Spec.Network.Service.Type = &npType
			proxy.Spec.Network.Service.NodePorts = []proxyv1alpha1.NodePortSpec{
				{Name: "http", Port: 8080, NodePort: 30080},
				{Name: "https", Port: 8443, NodePort: 30443},
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
								{Name: "https", Port: 8443},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))

			// Find the ports and verify NodePort assignments
			var httpPort, httpsPort *corev1.ServicePort
			for i := range svc.Spec.Ports {
				if svc.Spec.Ports[i].Name == "http" {
					httpPort = &svc.Spec.Ports[i]
				}
				if svc.Spec.Ports[i].Name == "https" {
					httpsPort = &svc.Spec.Ports[i]
				}
			}
			Expect(httpPort).ShouldNot(BeNil())
			Expect(httpPort.NodePort).Should(Equal(int32(30080)))
			Expect(httpsPort).ShouldNot(BeNil())
			Expect(httpsPort.NodePort).Should(Equal(int32(30443)))
		})

		It("creates service with LoadBalancer source ranges", func() {
			lbType := corev1.ServiceTypeLoadBalancer
			proxy.Spec.Network.Service.Type = &lbType
			proxy.Spec.Network.Service.LoadBalancerSourceRanges = []string{
				"10.0.0.0/8",
				"192.168.0.0/16",
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Spec.LoadBalancerSourceRanges).Should(ContainElement("10.0.0.0/8"))
			Expect(svc.Spec.LoadBalancerSourceRanges).Should(ContainElement("192.168.0.0/16"))
		})

		It("creates service with LoadBalancerClass", func() {
			lbType := corev1.ServiceTypeLoadBalancer
			proxy.Spec.Network.Service.Type = &lbType
			proxy.Spec.Network.Service.LoadBalancerClass = ptr.To("metallb.io/metallb")

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(*svc.Spec.LoadBalancerClass).Should(Equal("metallb.io/metallb"))
		})

		It("creates service with ClusterIP", func() {
			proxy.Spec.Network.Service.ClusterIP = "10.96.0.100"

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Spec.ClusterIP).Should(Equal("10.96.0.100"))
		})

		It("creates service with InternalTrafficPolicy", func() {
			internalPolicy := corev1.ServiceInternalTrafficPolicyLocal
			proxy.Spec.Network.Service.InternalTrafficPolicy = &internalPolicy

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(*svc.Spec.InternalTrafficPolicy).Should(Equal(corev1.ServiceInternalTrafficPolicyLocal))
		})

		It("creates service with AllocateLoadBalancerNodePorts false", func() {
			lbType := corev1.ServiceTypeLoadBalancer
			proxy.Spec.Network.Service.Type = &lbType
			proxy.Spec.Network.Service.AllocateLoadBalancerNodePorts = ptr.To(false)

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(*svc.Spec.AllocateLoadBalancerNodePorts).Should(BeFalse())
		})

		It("creates service with HealthCheckNodePort", func() {
			lbType := corev1.ServiceTypeLoadBalancer
			proxy.Spec.Network.Service.Type = &lbType
			proxy.Spec.Network.Service.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
			proxy.Spec.Network.Service.HealthCheckNodePort = 32000

			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).WithStatusSubresource(initObjs...).Build()
			r := Reconciler{
				Client: cli,
				Scheme: scheme,
			}

			frontends := &configv1alpha1.FrontendList{
				Items: []configv1alpha1.Frontend{
					{
						Spec: configv1alpha1.FrontendSpec{
							Binds: []configv1alpha1.Bind{
								{Name: "http", Port: 8080},
							},
						},
					},
				},
			}
			listens := &configv1alpha1.ListenList{}

			err := r.reconcileService(ctx, proxy, listens, frontends)
			Expect(err).ShouldNot(HaveOccurred())

			svc := &corev1.Service{}
			Expect(cli.Get(ctx, client.ObjectKey{Namespace: proxy.Namespace, Name: "test-instance-haproxy"}, svc)).ShouldNot(HaveOccurred())
			Expect(svc.Spec.HealthCheckNodePort).Should(Equal(int32(32000)))
		})
	})
})
