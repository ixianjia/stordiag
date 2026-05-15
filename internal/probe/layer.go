package probe

import "time"

type Layer int

const (
	LayerApp     Layer = 1 // 应用层：端到端操作延迟、吞吐
	LayerNetwork Layer = 2 // 网络层：TCP/TLS/DNS 耗时分解
	LayerSystem  Layer = 3 // 系统层：磁盘 IO、CPU iowait、内存压力
)

func (l Layer) String() string {
	switch l {
	case LayerApp:
		return "L1:app"
	case LayerNetwork:
		return "L2:network"
	case LayerSystem:
		return "L3:system"
	default:
		return "unknown"
	}
}

type ProbeResult struct {
	Layer   Layer
	Name    string  // 探针名，如 dns_lookup
	Status  string  // OK / WARN / FAIL
	Latency time.Duration
	Value   string
	Detail  string
	Err     error
}

type LayerReport struct {
	Layer   Layer
	Results []ProbeResult
}

func (r *LayerReport) Add(name, status string, lat time.Duration, value, detail string) {
	r.Results = append(r.Results, ProbeResult{
		Layer: r.Layer, Name: name, Status: status,
		Latency: lat, Value: value, Detail: detail,
	})
}

func (r *LayerReport) Error(name string, lat time.Duration, err error) {
	r.Results = append(r.Results, ProbeResult{
		Layer: r.Layer, Name: name, Status: "FAIL",
		Latency: lat, Detail: err.Error(), Err: err,
	})
}

func (r *LayerReport) Summary() (ok, warn, fail int) {
	for _, p := range r.Results {
		switch p.Status {
		case "OK":
			ok++
		case "WARN":
			warn++
		case "N/A":
			// skip
		default:
			fail++
		}
	}
	return
}

func MergeReports(reports ...*LayerReport) []LayerReport {
	var out []LayerReport
	for _, r := range reports {
		if r != nil {
			out = append(out, *r)
		}
	}
	return out
}
