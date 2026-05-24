package main

// Realistic project templates per technology stack. The seeder picks
// one template per project and uses its branches/tags/suite-names so
// the resulting dataset looks like a real CI fleet rather than
// `seed-project-NN`.

type Category struct {
	Slug       string   // url-safe id prefix (e.g. "payments")
	Name       string   // display name (e.g. "payments-service")
	Team       string
	Framework  string   // test framework (junit, jest, ...)
	Branches   []string
	Tags       []string // tag names this kind of project tends to attach
	SuiteNames []string // realistic-looking suites
	SpecNames  []string // realistic-looking specs (used for failing ones)
	ErrorPool  []string // error_message strings drawn at random
}

// Each category appears once per N projects so the distribution is
// roughly even when SEED_PROJECTS is large. Order matters for the
// modulo distribution; categories first in the list bias toward
// lower-numbered project IDs.
var Categories = []Category{
	// ---- Java / Spring Boot ----
	{
		Slug: "java", Name: "java-service", Team: "platform-backend",
		Framework: "junit",
		Branches:  []string{"main", "main", "main", "develop", "release/24.10", "release/24.11", "hotfix/auth-cve"},
		Tags:      []string{"java", "junit", "spring-boot", "unit", "integration", "p0", "p1", "slow"},
		SuiteNames: []string{
			"PaymentControllerIT", "AuthServiceTest", "UserRepositoryIT",
			"BillingAggregatorTest", "OrderSagaIT", "MetricsExporterTest",
			"SecurityConfigTest", "KafkaConsumerIT", "RedisCacheTest",
			"FlywayMigrationTest",
		},
		SpecNames: []string{
			"shouldCreatePaymentWhenAmountValid",
			"shouldRejectAuthWhenTokenExpired",
			"shouldRetryOnTransientDbError",
			"shouldCloseOrderOnRefund",
			"shouldEmitMetricOnFailure",
			"shouldUpgradeSchemaIdempotently",
		},
		ErrorPool: []string{
			"org.springframework.dao.DataIntegrityViolationException: duplicate key violates unique constraint \"users_email_key\"",
			"java.net.SocketTimeoutException: Read timed out after 30000ms calling https://billing.internal",
			"org.opentest4j.AssertionFailedError: expected: <200 OK> but was: <502 Bad Gateway>",
			"java.lang.NullPointerException: Cannot invoke \"User.getRole()\" because \"user\" is null",
			"org.springframework.beans.factory.UnsatisfiedDependencyException: bean 'paymentService' not loaded",
		},
	},
	// ---- Infrastructure / Terraform ----
	{
		Slug: "infra", Name: "platform-infra", Team: "platform-sre",
		Framework: "terratest",
		Branches:  []string{"main", "main", "develop", "feature/aws-region-failover", "feature/k8s-1.30"},
		Tags:      []string{"infra", "terraform", "aws", "gcp", "terratest", "smoke", "destructive"},
		SuiteNames: []string{
			"VpcBaselineTest", "EksClusterUpgradeTest", "S3LifecycleTest",
			"IamRolePolicyTest", "RdsBackupRotationTest", "CloudfrontEdgeTest",
			"NetworkPeeringTest", "DnsResolutionTest",
		},
		SpecNames: []string{
			"vpc_has_three_az_subnets",
			"eks_node_group_uses_spot",
			"s3_buckets_block_public_access",
			"iam_role_passes_least_privilege",
			"rds_backups_retained_30_days",
			"dns_resolves_within_300ms",
		},
		ErrorPool: []string{
			"Error: timeout while waiting for state to become 'available' on aws_db_instance.prod after 40m0s",
			"Error: Plan has diff after Apply — possible drift in aws_iam_policy.app_role",
			"Error: ResourceNotReady: instance i-0abcd1234ef status was 'pending', wanted 'running'",
			"AssertionError: expected DNS record fern.example.com to exist, none found in zone Z123ABCDE",
		},
	},
	// ---- FluxCD ----
	{
		Slug: "flux", Name: "flux-system", Team: "platform-gitops",
		Framework: "kuttl",
		Branches:  []string{"main", "main", "main", "stage", "prod"},
		Tags:      []string{"gitops", "fluxcd", "kuttl", "kubernetes", "reconcile", "smoke"},
		SuiteNames: []string{
			"KustomizationReconcileTest", "HelmReleaseSyncTest",
			"GitRepoSourceTest", "OCIRepoSourceTest",
			"ImageAutomationTest", "NotificationProviderTest",
		},
		SpecNames: []string{
			"kustomization_reconciles_within_2m",
			"helmrelease_rolls_back_on_failure",
			"gitrepository_picks_up_new_revision",
			"image_automation_writes_back_to_repo",
			"notification_fires_on_suspended_resource",
		},
		ErrorPool: []string{
			"timed out waiting for Kustomization fern-apps in fern-system to become Ready=True (status=False, reason=BuildFailed)",
			"HelmRelease fern-app upgrade failed: another operation (install/upgrade/rollback) is in progress",
			"Kustomization build failed: accumulating resources: open base.yaml: no such file or directory",
			"GitRepository fern-config: unable to clone, ssh: handshake failed: knownhosts: key mismatch",
		},
	},
	// ---- Helm charts ----
	{
		Slug: "helm", Name: "helm-charts", Team: "platform-deploy",
		Framework: "helm-unittest",
		Branches:  []string{"main", "main", "release/v1", "release/v2"},
		Tags:      []string{"helm", "helm-unittest", "chart", "lint", "render", "smoke"},
		SuiteNames: []string{
			"ChartLintTest", "ValuesRenderTest", "DeploymentRenderTest",
			"ServiceMonitorRenderTest", "HpaRenderTest", "PdbRenderTest",
			"NetworkPolicyRenderTest", "IngressRenderTest",
		},
		SpecNames: []string{
			"renders_with_default_values",
			"renders_with_minimal_values",
			"renders_pdb_when_replicas_gt_1",
			"renders_hpa_when_autoscaling_enabled",
			"renders_servicemonitor_when_metrics_enabled",
			"linter_reports_no_warnings",
		},
		ErrorPool: []string{
			"Error: template: chart/templates/deployment.yaml:35:14: executing \"chart/templates/deployment.yaml\" at <.Values.image.tag>: nil pointer evaluating",
			"Error: values don't meet the specifications of the schema(s) in the following chart(s): redis: replicaCount must be >= 1",
			"Error: helm-unittest: spec failed: expected resource count to be 3 but got 4",
			"Error: chart lint: [ERROR] templates/: unable to parse YAML: error converting YAML to JSON",
		},
	},
	// ---- Node.js / web ----
	{
		Slug: "web", Name: "web-app", Team: "frontend",
		Framework: "jest",
		Branches:  []string{"main", "main", "main", "develop", "feature/new-checkout", "feature/i18n"},
		Tags:      []string{"nodejs", "jest", "react", "e2e", "playwright", "cypress", "a11y", "visual"},
		SuiteNames: []string{
			"CartReducerTest", "CheckoutFlowE2E", "LoginFormTest",
			"OrderHistoryTest", "ProductCardSnapshot", "AccessibilityAudit",
			"VisualRegression", "ApiClientTest",
		},
		SpecNames: []string{
			"renders the cart with three items",
			"redirects to login when checkout requires auth",
			"axe reports no a11y violations on /home",
			"clicking purchase fires analytics event",
			"product card respects prefers-reduced-motion",
			"order history paginates",
		},
		ErrorPool: []string{
			"TypeError: Cannot read properties of undefined (reading 'price') at Cart.render (src/Cart.tsx:42:18)",
			"Timeout - Async callback was not invoked within the 30000 ms timeout specified by jest.setTimeout.",
			"AssertionError: expected document.title to equal 'Checkout' but got 'Home'",
			"PlaywrightError: locator.click: Target page, context or browser has been closed",
			"axe-core: [color-contrast] Elements must meet minimum color contrast ratio thresholds (1 found)",
		},
	},
}

// PickCategory deterministically picks a category for project index i.
func PickCategory(i int) Category {
	return Categories[i%len(Categories)]
}
