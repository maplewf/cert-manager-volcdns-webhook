package volcengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/volcengine/volcengine-go-sdk/service/dns"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	corev1 "k8s.io/api/core/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config structure matching the JSON config
type Config struct {
	Region             string                   `json:"region"`
	ZoneID             string                   `json:"zoneID,omitempty"`
	RoleTRN            string                   `json:"roleTrn,omitempty"`
	AccessKeySecretRef corev1.SecretKeySelector `json:"accessKeySecretRef"`
	SecretKeySecretRef corev1.SecretKeySelector `json:"secretKeySecretRef"`
}

type Solver struct {
	client *kubernetes.Clientset
}

func NewSolver() *Solver {
	return &Solver{}
}

func (s *Solver) Name() string {
	return "volcdns-resolver"
}

func (s *Solver) Present(ch *v1alpha1.ChallengeRequest) error {
	logrus.Infof("Presenting challenge for %s", ch.ResolvedFQDN)
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	creds, err := s.getCredentials(cfg, ch.ResourceNamespace)
	if err != nil {
		return err
	}

	client, err := NewDNSWrapper(cfg.Region, creds)
	if err != nil {
		return err
	}

	// Find zone
	zoneID, err := s.getZoneID(client, cfg, ch.ResolvedFQDN)
	if err != nil {
		return err
	}

	// Create record
	// host is the subdomain part relative to zone
	host := extractHost(ch.ResolvedFQDN, zoneID.Name)

	// Volcengine requires TXT value to be quoted if it contains spaces or special chars?
	// ACME challenge token is usually alphanumeric.
	// Use escapeTXTRecordValue to handle potential quoting requirements
	val := escapeTXTRecordValue(ch.Key)

	return client.CreateRecord(context.Background(), zoneID.ID, host, "TXT", val, 300)
}

func (s *Solver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	logrus.Infof("Cleaning up challenge for %s", ch.ResolvedFQDN)
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	creds, err := s.getCredentials(cfg, ch.ResourceNamespace)
	if err != nil {
		return err
	}

	client, err := NewDNSWrapper(cfg.Region, creds)
	if err != nil {
		return err
	}

	zoneID, err := s.getZoneID(client, cfg, ch.ResolvedFQDN)
	if err != nil {
		return err
	}

	host := extractHost(ch.ResolvedFQDN, zoneID.Name)

	// Find the record to delete
	records, err := client.GetRecords(context.Background(), zoneID.ID, host)
	if err != nil {
		return err
	}

	val := escapeTXTRecordValue(ch.Key)

	for _, r := range records {
		if StringValue(r.Type) == "TXT" && StringValue(r.Host) == host {
			// Volcengine API might return quoted value, so we might need to unescape or compare carefully
			// Let's check both raw and unescaped
			rVal := StringValue(r.Value)
			if rVal == val || unescapeTXTRecordValue(rVal) == ch.Key {
				return client.DeleteRecord(context.Background(), zoneID.ID, StringValue(r.RecordID))
			}
		}
	}

	logrus.Warnf("Record not found for cleanup: %s", ch.ResolvedFQDN)
	return nil
}

func (s *Solver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	s.client = cl
	return nil
}

func loadConfig(cfgJSON *extapi.JSON) (Config, error) {
	cfg := Config{}
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}
	return cfg, nil
}

func (s *Solver) getCredentials(cfg Config, namespace string) (*credentials.Credentials, error) {
	// If AK/SK provided in secret ref, use them
	if cfg.AccessKeySecretRef.Name != "" && cfg.SecretKeySecretRef.Name != "" {
		ak, err := s.getSecret(cfg.AccessKeySecretRef, namespace)
		if err != nil {
			return nil, err
		}
		sk, err := s.getSecret(cfg.SecretKeySecretRef, namespace)
		if err != nil {
			return nil, err
		}
		return credentials.NewStaticCredentials(ak, sk, ""), nil
	}

	// Otherwise try IRSA (env vars or configured role)
	p := credentials.NewOIDCCredentialsProviderFromEnv()
	if cfg.RoleTRN != "" {
		// If RoleTRN is specified in config, override the one from env
		p.RoleTrn = cfg.RoleTRN
	}
	if p.Endpoint == "" {
		p.Endpoint = "sts.volcengineapi.com"
	}
	if p.RoleSessionName == "" {
		p.RoleSessionName = "cert-manager"
	}
	if p.OIDCTokenFilePath == "" {
		p.OIDCTokenFilePath = "/var/run/secrets/vke.volcengine.com/irsa-tokens/token"
	}
	return credentials.NewCredentials(p), nil
}

func (s *Solver) getSecret(selector corev1.SecretKeySelector, namespace string) (string, error) {
	secret, err := s.client.CoreV1().Secrets(namespace).Get(context.Background(), selector.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %v", namespace, selector.Name, err)
	}

	bytes, ok := secret.Data[selector.Key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s does not contain key %s", namespace, selector.Name, selector.Key)
	}
	return string(bytes), nil
}

type ZoneInfo struct {
	ID   int64
	Name string
}

func (s *Solver) getZoneID(client *DNSWrapper, cfg Config, fqdn string) (ZoneInfo, error) {
	if cfg.ZoneID != "" {
		// If ZoneID is configured, verify it exists and get its name
		// We can use ListZones or just assume the user is right, but getting the name is useful for extractHost
		// The API doesn't seem to support GetZone(id) directly based on our client wrapper, so let's list.
		// Or we can try to fetch records directly if we trust the ID. But extractHost needs zoneName.
		// Let's assume we still need to find the zone name corresponding to this ID.

		zid, err := strconv.ParseInt(cfg.ZoneID, 10, 64)
		if err != nil {
			return ZoneInfo{}, fmt.Errorf("invalid zoneID format: %v", err)
		}

		// Ideally we should cache this or have a better way to get zone name
		zones, err := client.ListZones(context.Background())
		if err != nil {
			return ZoneInfo{}, err
		}

		for _, z := range zones {
			if int64(Int32Value(z.ZID)) == zid {
				return ZoneInfo{ID: zid, Name: StringValue(z.ZoneName)}, nil
			}
		}

		return ZoneInfo{}, fmt.Errorf("configured zoneID %s not found", cfg.ZoneID)
	}

	return s.findZoneID(client, fqdn)
}

func (s *Solver) findZoneID(client *DNSWrapper, fqdn string) (ZoneInfo, error) {
	zones, err := client.ListZones(context.Background())
	if err != nil {
		return ZoneInfo{}, err
	}

	// Find the longest matching zone
	var match *dns.ZoneForListZonesOutput
	longestLen := 0

	fqdn = strings.TrimSuffix(fqdn, ".")

	for _, z := range zones {
		zoneName := StringValue(z.ZoneName)
		zoneName = strings.TrimSuffix(zoneName, ".")

		if fqdn == zoneName || strings.HasSuffix(fqdn, "."+zoneName) {
			if len(zoneName) > longestLen {
				match = z
				longestLen = len(zoneName)
			}
		}
	}

	if match == nil {
		return ZoneInfo{}, fmt.Errorf("no matching zone found for %s", fqdn)
	}

	return ZoneInfo{ID: int64(Int32Value(match.ZID)), Name: StringValue(match.ZoneName)}, nil
}

func extractHost(fqdn, zoneName string) string {
	fqdn = strings.TrimSuffix(fqdn, ".")
	zoneName = strings.TrimSuffix(zoneName, ".")

	if fqdn == zoneName {
		return "@"
	}

	if strings.HasSuffix(fqdn, "."+zoneName) {
		return strings.TrimSuffix(fqdn, "."+zoneName)
	}

	return fqdn
}
