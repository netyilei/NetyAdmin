// manifest.go：module.yaml 解析与校验（CI check-manifests 的运行时对偶）。
package module

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Kind 是模块类别（§13.2；kernel 枚举值仅为 CI/运行时对偶一致，基座运行时模块不使用）。
type Kind string

const (
	KindKernel          Kind = "kernel"
	KindPlatformService Kind = "platform-service"
	KindBusiness        Kind = "business"
	KindEdge            Kind = "edge"
)

// ScalingMode 是伸缩模式枚举（AGENTS §3）。
type ScalingMode string

const (
	ScalingStateless ScalingMode = "stateless"
	ScalingLeader    ScalingMode = "leader"
	ScalingSharded   ScalingMode = "sharded"
	ScalingStateful  ScalingMode = "stateful"
)

// IdempotencyKey 端口方法幂等键要求枚举（§25.5）。
const (
	IdemKeyNone     = "none"
	IdemKeyOptional = "optional"
	IdemKeyRequired = "required"
)

// PortMethod 是端口方法元数据（RPC 客户端按此执行幂等/超时/重试）。
type PortMethod struct {
	Name           string `yaml:"name"`
	Idempotent     bool   `yaml:"idempotent"`
	Timeout        string `yaml:"timeout"` // duration 字符串，如 200ms
	Retryable      bool   `yaml:"retryable"`
	MaxRetries     int    `yaml:"max_retries"`
	IdempotencyKey string `yaml:"idempotency_key"` // none | optional | required
}

// Port 是模块对外端口声明（§13.2）。
type Port struct {
	Name    string       `yaml:"name"` // "<module>.<Service>"，全局唯一
	Methods []PortMethod `yaml:"methods"`
}

// JobDecl 是模块 Job 声明（§13.2：name+trigger；spec 细节在模块代码， scheduler 消费）。
type JobDecl struct {
	Name    string `yaml:"name"`
	Trigger string `yaml:"trigger"` // cron | interval | once | delay | event | queue
}

// Events 是模块事件声明（Topic 常量在模块 api/events）。
type Events struct {
	Publishes  []string `yaml:"publishes"`
	Subscribes []string `yaml:"subscribes"`
}

// Scaling 是伸缩声明（AGENTS §3）。
type Scaling struct {
	Mode     ScalingMode `yaml:"mode"` // stateless | leader | sharded | stateful
	Requires []string    `yaml:"requires"`
}

// Deployment 是部署声明。
type Deployment struct {
	ColocateWith      string `yaml:"colocate_with"`
	ExposeInternalRPC bool   `yaml:"expose_internal_rpc"`
}

// Manifest 是模块自描述（字段集；校验规则与 CI check-manifests 对偶）。
type Manifest struct {
	Name               string `yaml:"name"`
	Kind               Kind   `yaml:"kind"`
	Version            string `yaml:"version"`
	APIVersion         int    `yaml:"api_version"`
	ContractVersion    string `yaml:"contract_version"`
	MinKernelVersion   string `yaml:"min_kernel_version"`
	Owner              string `yaml:"owner"`
	DataClassification string `yaml:"data_classification"`
	SLO                struct {
		LatencyP99MS int     `yaml:"latency_p99_ms"`
		Availability float64 `yaml:"availability"`
	} `yaml:"slo"`
	DependsOn         []string       `yaml:"depends_on"`
	OptionalDependsOn []string       `yaml:"optional_depends_on"`
	Ports             []Port         `yaml:"ports"`
	Events            Events         `yaml:"events"`
	Jobs              []JobDecl      `yaml:"jobs"`
	Tables            []string       `yaml:"tables"`
	ConfigSection     string         `yaml:"config_section"`
	OptionalFeatures  map[string]any `yaml:"optional_features"`
	Scaling           Scaling        `yaml:"scaling"`
	Deployment        Deployment     `yaml:"deployment"`
}

// LoadManifest 从模块目录读取并解析 module.yaml（KnownFields：未知字段即失败，防拼写静默吞）。
func LoadManifest(dir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "module.yaml"))
	if err != nil {
		return nil, fmt.Errorf("module: read manifest: %w", err)
	}
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	var mf Manifest
	if err := dec.Decode(&mf); err != nil {
		return nil, fmt.Errorf("module: parse manifest: %w", err)
	}
	if err := mf.Validate(); err != nil {
		return nil, err
	}
	return &mf, nil
}

// Validate 校验单模块 Manifest（字段/枚举/格式/元数据一致性；跨模块校验在 Registry.Build）。
func (m *Manifest) Validate() error {
	var errs []error
	if m.Name == "" {
		errs = append(errs, errors.New("missing name"))
	}
	switch m.Kind {
	case KindKernel, KindPlatformService, KindBusiness, KindEdge:
	default:
		errs = append(errs, fmt.Errorf("invalid kind %q (kernel|platform-service|business|edge)", m.Kind))
	}
	if m.Version == "" {
		errs = append(errs, errors.New("missing version"))
	}
	if m.APIVersion <= 0 {
		errs = append(errs, errors.New("missing api_version"))
	}
	if m.Owner == "" {
		errs = append(errs, errors.New("missing owner"))
	}
	if m.ConfigSection == "" {
		errs = append(errs, errors.New("missing config_section"))
	}
	switch m.Scaling.Mode {
	case ScalingStateless, ScalingLeader, ScalingSharded, ScalingStateful:
	default:
		errs = append(errs, fmt.Errorf("invalid scaling.mode %q (AGENTS §3)", m.Scaling.Mode))
	}
	switch m.DataClassification {
	case "", "public", "internal", "confidential", "pii":
	default:
		errs = append(errs, fmt.Errorf("invalid data_classification %q", m.DataClassification))
	}
	if m.SLO.LatencyP99MS < 0 {
		errs = append(errs, errors.New("slo.latency_p99_ms must be >= 0"))
	}
	if m.SLO.Availability < 0 || m.SLO.Availability > 1 {
		errs = append(errs, errors.New("slo.availability must be within [0,1]"))
	}
	seenPort := map[string]bool{}
	for _, p := range m.Ports {
		if p.Name == "" {
			errs = append(errs, errors.New("port missing name"))
			continue
		}
		if !strings.HasPrefix(p.Name, m.Name+".") {
			errs = append(errs, fmt.Errorf("port %q must be prefixed with module name (<module>.<Service>)", p.Name))
		}
		if seenPort[p.Name] {
			errs = append(errs, fmt.Errorf("port %q duplicated", p.Name))
		}
		seenPort[p.Name] = true
		for _, meth := range p.Methods {
			if meth.Name == "" {
				errs = append(errs, fmt.Errorf("port %q method missing name", p.Name))
				continue
			}
			switch meth.IdempotencyKey {
			case "", IdemKeyNone, IdemKeyOptional, IdemKeyRequired:
			default:
				errs = append(errs, fmt.Errorf("port %q.%s invalid idempotency_key %q (none|optional|required)", p.Name, meth.Name, meth.IdempotencyKey))
			}
			// §25.5 语义一致性：可重试必须幂等或有幂等键
			if meth.Retryable && !meth.Idempotent && meth.IdempotencyKey != IdemKeyRequired {
				errs = append(errs, fmt.Errorf("port %q.%s retryable requires idempotent or idempotency_key=required", p.Name, meth.Name))
			}
			if meth.Timeout != "" {
				if _, err := time.ParseDuration(meth.Timeout); err != nil {
					errs = append(errs, fmt.Errorf("port %q.%s invalid timeout %q", p.Name, meth.Name, meth.Timeout))
				}
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("manifest %s: %w", m.Name, errors.Join(errs...))
	}
	return nil
}
