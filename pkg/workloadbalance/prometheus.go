package workloadbalance

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"encoding/json"
)

type PrometheusHandle struct {
	Address string
}

type NodeMetrics struct {
	CPUUsage     float64
	MemUsage     float64
	IOStorage    float64
	IONetwork    float64
	MaxCPU       float64
	MaxMem       float64
	MaxIOStorage float64
	MaxIONetwork float64
}

type PodMetrics struct {
	CPUUsage     float64
	MemUsage     float64
	IOStorage    float64
	IONetwork    float64
}


func NewPrometheus(address string) *PrometheusHandle {
	return &PrometheusHandle{Address: address}
}

func (p *PrometheusHandle) GetNodeMetrics(nodeName string) (*NodeMetrics, error) {
	url := fmt.Sprintf("%s/api/v1/query?query=node:metrics:%s", p.Address, nodeName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Erreur requête Prometheus: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal(body, &data)

	// Exemple : extraire les métriques de la réponse JSON
	metrics := &NodeMetrics{
		CPUUsage:     extractMetric(data, "cpu"),
		MemUsage:     extractMetric(data, "memory"),
		IOStorage:    extractMetric(data, "io_storage"),
		IONetwork:    extractMetric(data, "io_network"),
		MaxCPU:       100, // Ex. 100% CPU max
		MaxMem:       100, // Ex. 100% mémoire max
		MaxIOStorage: 500, // Ex. 500 MB/s max
		MaxIONetwork: 1000, // Ex. 1 Gbps max
	}

	return metrics, nil
}

// GetPodMetrics récupère les métriques d’un pod donné via Prometheus
func (p *PrometheusHandle) GetPodMetrics(podName, namespace string) (*PodMetrics, error) {
	// 🔹 Construire les requêtes pour les métriques CPU, mémoire, IO
	queries := map[string]string{
		"cpu":       fmt.Sprintf("sum(rate(container_cpu_usage_seconds_total{pod='%s', namespace='%s'}[5m]))", podName, namespace),
		"memory":    fmt.Sprintf("sum(container_memory_usage_bytes{pod='%s', namespace='%s'})", podName, namespace),
		"io_storage": fmt.Sprintf("sum(rate(container_fs_reads_bytes_total{pod='%s', namespace='%s'}[5m]) + rate(container_fs_writes_bytes_total{pod='%s', namespace='%s'}[5m]))", podName, namespace, podName, namespace),
		"io_network": fmt.Sprintf("sum(rate(container_network_transmit_bytes_total{pod='%s', namespace='%s'}[5m]) + rate(container_network_receive_bytes_total{pod='%s', namespace='%s'}[5m]))", podName, namespace, podName, namespace),
	}

	// 🔹 Récupération des métriques via Prometheus
	metrics := &PodMetrics{}
	for metric, query := range queries {
		value, err := p.queryPrometheus(query)
		if err != nil {
			return nil, fmt.Errorf("erreur récupération %s pour le pod %s: %v", metric, podName, err)
		}
		switch metric {
		case "cpu":
			metrics.CPUUsage = value
		case "memory":
			metrics.MemUsage = value / (1024 * 1024) // Conversion en MiB
		case "io_storage":
			metrics.IOStorage = value / (1024 * 1024) // Conversion en MiB/s
		case "io_network":
			metrics.IONetwork = value / (1024 * 1024) // Conversion en Mbps
		}
	}

	return metrics, nil
}

// queryPrometheus exécute une requête PromQL et retourne la valeur obtenue
func (p *PrometheusHandle) queryPrometheus(query string) (float64, error) {
	url := fmt.Sprintf("%s/api/v1/query?query=%s", p.Address, query)
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("échec de la requête Prometheus: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("échec de lecture de la réponse: %v", err)
	}

	// 🔹 Parsing de la réponse JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, fmt.Errorf("erreur parsing JSON: %v", err)
	}

	// 🔹 Extraction de la valeur numérique
	results, found := data["data"].(map[string]interface{})["result"].([]interface{})
	if !found || len(results) == 0 {
		return 0, fmt.Errorf("aucune donnée trouvée pour la requête")
	}

	firstResult := results[0].(map[string]interface{})["value"].([]interface{})
	value, err := extractFloat(firstResult[1])
	if err != nil {
		return 0, fmt.Errorf("échec conversion valeur Prometheus: %v", err)
	}

	return value, nil
}

// extractFloat convertit une valeur Prometheus JSON en float64
func extractFloat(v interface{}) (float64, error) {
	strVal, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("échec extraction valeur: type incorrect")
	}
	var floatVal float64
	_, err := fmt.Sscanf(strVal, "%f", &floatVal)
	if err != nil {
		return 0, fmt.Errorf("échec conversion float: %v", err)
	}
	return floatVal, nil
}


func extractMetric(data map[string]interface{}, metric string) float64 {
	// Extraction d'une métrique de la réponse JSON de Prometheus
	return 50.0 // Valeur bidon, ici tu parses correctement le JSON
}
