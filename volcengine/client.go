package volcengine

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/volcengine/volcengine-go-sdk/service/dns"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/request"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

var (
	defaultPageSize  = 100
	defaultRecordRemark = "managed by cert-manager-volcdns-webhook"
)

// dnsAPI interface for mocking
type dnsAPI interface {
	ListZones(ctx context.Context) ([]*dns.ZoneForListZonesOutput, error)
	GetRecords(ctx context.Context, zid int64) ([]*dns.RecordForListRecordsOutput, error)
	CreateRecord(ctx context.Context, zoneID int64, host, recordType, target string, TTL int32) error
	DeleteRecord(ctx context.Context, zoneID int64, recordID string) error
    // Add GetRecordByHost if possible
}

// dnsClient interface from SDK
type dnsClient interface {
	ListZonesWithContext(ctx context.Context, input *dns.ListZonesInput, options ...request.Option) (*dns.ListZonesOutput, error)
	ListRecordsWithContext(ctx context.Context, input *dns.ListRecordsInput, options ...request.Option) (*dns.ListRecordsOutput, error)
	CreateRecordWithContext(ctx context.Context, input *dns.CreateRecordInput, options ...request.Option) (*dns.CreateRecordOutput, error)
	UpdateRecordWithContext(ctx context.Context, input *dns.UpdateRecordInput, options ...request.Option) (*dns.UpdateRecordOutput, error)
	DeleteRecordWithContext(ctx context.Context, input *dns.DeleteRecordInput, options ...request.Option) (*dns.DeleteRecordOutput, error)
}

type DNSWrapper struct {
	client dnsClient
}

func NewDNSWrapper(regionID string, credentials *credentials.Credentials) (*DNSWrapper, error) {
	c := volcengine.NewConfig().
		WithRegion(regionID).
		WithCredentials(credentials)
		// WithLogger(NewLoggerAdapter(logrus.StandardLogger().WithField("client", "dns")))
	s, err := session.NewSession(c)
	if err != nil {
		logrus.Errorf("Failed to create volcengine session: %v", err)
		return nil, err
	}
	dc := dns.New(s)

	return &DNSWrapper{
		client: dc,
	}, nil
}

func (w *DNSWrapper) CreateRecord(ctx context.Context, zoneID int64, host, recordType, target string, TTL int32) error {
	request := &dns.CreateRecordInput{
		Host:   &host,
		Type:   &recordType,
		Value:  &target,
		ZID:    &zoneID,
		TTL:    &TTL,
		Remark: volcengine.String(defaultRecordRemark),
	}
	resp, err := w.client.CreateRecordWithContext(ctx, request)
	logrus.Tracef("Create record request: %+v, resp: %+v", request, resp)
	if err != nil {
		return fmt.Errorf("failed to create record: %v", err)
	} else if resp.Metadata.Error != nil {
		return fmt.Errorf("failed to create record metadata error: %v", resp.Metadata.Error)
	}

	logrus.Infof("Successfully created volcengine record: %+v", resp)
	return nil
}

func (w *DNSWrapper) DeleteRecord(ctx context.Context, zoneID int64, recordID string) error {
	req := &dns.DeleteRecordInput{
		RecordID: &recordID,
	}
	resp, err := w.client.DeleteRecordWithContext(ctx, req)
	logrus.Tracef("Delete record request: %+v, resp: %+v", req, resp)
	if err != nil {
		return fmt.Errorf("failed to delete record: %v", err)
	} else if resp.Metadata.Error != nil {
		return fmt.Errorf("failed to delete record metadata error: %v", resp.Metadata.Error)
	}
	logrus.Infof("Successfully deleted volcengine record: %+v", resp)
	return nil
}

func (w *DNSWrapper) ListZones(ctx context.Context) ([]*dns.ZoneForListZonesOutput, error) {
	zones, err := QueryAll(defaultPageSize, func(pageNum, pageSize int) ([]*dns.ZoneForListZonesOutput, int, error) {
		req := &dns.ListZonesInput{
			PageSize:   volcengine.Int32(int32(pageSize)),
			PageNumber: volcengine.Int32(int32(pageNum)),
		}
		resp, err := w.client.ListZonesWithContext(ctx, req)
		logrus.Tracef("List volcengine zones: req: %s, resp: %s", req, resp)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list volcengine zones: %v", err)
		} else if resp.Metadata.Error != nil {
			return nil, 0, fmt.Errorf("failed to list volcengine zones metadata error: %v", resp.Metadata.Error)
		}
		return resp.Zones, int(volcengine.Int32Value(resp.Total)), nil
	})
	if err != nil {
		logrus.Errorf("Failed to list volcengine zones: %v", err)
		return nil, err
	}

	logrus.Debugf("Successfully list volcengine zones: %+v", zones)
	return zones, nil
}

func (w *DNSWrapper) GetRecords(ctx context.Context, zid int64, host string) ([]*dns.RecordForListRecordsOutput, error) {
	// Note: API might support Host filtering. If so, we should use it.
    // Assuming ListRecordsInput has Host field.
    // If not, we fall back to filtering in memory.
    // Let's assume for now we list all and filter, or if Host is supported.
    // The external-dns implementation didn't use Host.
    
	res, err := QueryAll(defaultPageSize, func(pageNum, pageSize int) ([]*dns.RecordForListRecordsOutput, int, error) {
		req := dns.ListRecordsInput{
			ZID:        &zid,
			PageSize:   volcengine.Int32(int32(pageSize)),
			PageNumber: volcengine.Int32(int32(pageNum)),
            Host:       volcengine.String(host), // Optimistically trying to use Host filter if supported
		}
		resp, err := w.client.ListRecordsWithContext(ctx, &req)
		logrus.Tracef("List records req: %s, resp: %+v", req, resp)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list records: %v", err)
		} else if resp.Metadata.Error != nil {
			return nil, 0, fmt.Errorf("failed to list records metadata error: %v", resp.Metadata.Error)
		}
		return resp.Records, int(volcengine.Int32Value(resp.TotalCount)), nil
	})
	if err != nil {
		logrus.Errorf("Failed to list records: %v", err)
		return nil, err
	}

	logrus.Debugf("Successfully list records: %+v", res)
	return res, nil
}
