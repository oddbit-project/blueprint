package franz

import (
	"context"
	"sync"

	"github.com/oddbit-project/blueprint/log"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// TopicConfig represents topic configuration for creation
type TopicConfig struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	Configs           map[string]*string // Topic-level configs (e.g., retention.ms)
}

// NewTopicConfig creates a new topic configuration
func NewTopicConfig(name string, partitions int32, replicationFactor int16) *TopicConfig {
	return &TopicConfig{
		Name:              name,
		Partitions:        partitions,
		ReplicationFactor: replicationFactor,
	}
}

// WithConfig adds a configuration option to the topic
func (t *TopicConfig) WithConfig(key, value string) *TopicConfig {
	if t.Configs == nil {
		t.Configs = make(map[string]*string)
	}
	t.Configs[key] = &value
	return t
}

// TopicInfo represents information about a topic
type TopicInfo struct {
	Name       string
	Partitions []PartitionInfo
	Internal   bool
}

// PartitionInfo represents information about a partition
type PartitionInfo struct {
	ID       int32
	Leader   int32
	Replicas []int32
	ISR      []int32
}

// BrokerInfo represents information about a broker
type BrokerInfo struct {
	ID   int32
	Host string
	Port int32
	Rack *string
}

// GroupInfo represents information about a consumer group
type GroupInfo struct {
	Name         string
	State        string
	ProtocolType string
	Protocol     string
	Members      []GroupMember
}

// GroupMember represents a member of a consumer group
type GroupMember struct {
	ID         string
	ClientID   string
	ClientHost string
}

// Admin is a Kafka admin client
type Admin struct {
	client      *kgo.Client
	adminClient *kadm.Client
	config      *AdminConfig
	Logger      *log.Logger

	mu     sync.RWMutex
	closed bool
}

// NewAdmin creates a new admin client
func NewAdmin(cfg *AdminConfig, logger *log.Logger) (*Admin, error) {
	if cfg == nil {
		cfg = DefaultAdminConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts, err := cfg.buildOpts()
	if err != nil {
		return nil, err
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = NewAdminLogger(cfg.Brokers)
	} else {
		logger = AdminLogger(logger, cfg.Brokers)
	}

	return &Admin{
		client:      client,
		adminClient: kadm.NewClient(client),
		config:      cfg,
		Logger:      logger,
	}, nil
}

// CreateTopics creates one or more topics
func (a *Admin) CreateTopics(ctx context.Context, topics ...*TopicConfig) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	for _, topic := range topics {
		a.Logger.Info("Creating topic", log.KV{
			"topic":             topic.Name,
			"partitions":        topic.Partitions,
			"replicationFactor": topic.ReplicationFactor,
		})

		resp, err := adminClient.CreateTopics(ctx, topic.Partitions, topic.ReplicationFactor, topic.Configs, topic.Name)
		if err != nil {
			a.Logger.Error(err, "Failed to create topic", log.KV{"topic": topic.Name})
			return err
		}
		for _, t := range resp {
			if t.Err != nil {
				a.Logger.Error(t.Err, "Failed to create topic", log.KV{"topic": t.Topic})
				return t.Err
			}
		}
	}
	return nil
}

// DeleteTopics deletes one or more topics
func (a *Admin) DeleteTopics(ctx context.Context, topics ...string) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	a.Logger.Info("Deleting topics", log.KV{"topics": topics})

	resp, err := adminClient.DeleteTopics(ctx, topics...)
	if err != nil {
		a.Logger.Error(err, "Failed to delete topics")
		return err
	}
	for _, t := range resp {
		if t.Err != nil {
			a.Logger.Error(t.Err, "Failed to delete topic", log.KV{"topic": t.Topic})
			return t.Err
		}
	}
	return nil
}

// ListTopics lists all topics
func (a *Admin) ListTopics(ctx context.Context) ([]TopicInfo, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return nil, ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	topics, err := adminClient.ListTopics(ctx)
	if err != nil {
		a.Logger.Error(err, "Failed to list topics")
		return nil, err
	}

	result := make([]TopicInfo, 0, len(topics))
	for _, t := range topics {
		info := TopicInfo{
			Name:     t.Topic,
			Internal: t.IsInternal,
		}
		for _, p := range t.Partitions {
			info.Partitions = append(info.Partitions, PartitionInfo{
				ID:       p.Partition,
				Leader:   p.Leader,
				Replicas: p.Replicas,
				ISR:      p.ISR,
			})
		}
		result = append(result, info)
	}
	return result, nil
}

// DescribeTopics returns detailed information about specific topics
func (a *Admin) DescribeTopics(ctx context.Context, topics ...string) ([]TopicInfo, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return nil, ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	described, err := adminClient.ListTopics(ctx, topics...)
	if err != nil {
		return nil, err
	}

	result := make([]TopicInfo, 0, len(described))
	for _, t := range described {
		info := TopicInfo{
			Name:     t.Topic,
			Internal: t.IsInternal,
		}
		for _, p := range t.Partitions {
			info.Partitions = append(info.Partitions, PartitionInfo{
				ID:       p.Partition,
				Leader:   p.Leader,
				Replicas: p.Replicas,
				ISR:      p.ISR,
			})
		}
		result = append(result, info)
	}
	return result, nil
}

// TopicExists checks if a topic exists
func (a *Admin) TopicExists(ctx context.Context, topic string) (bool, error) {
	topics, err := a.ListTopics(ctx)
	if err != nil {
		return false, err
	}
	for _, t := range topics {
		if t.Name == topic {
			return true, nil
		}
	}
	return false, nil
}

// ListBrokers lists all brokers in the cluster
func (a *Admin) ListBrokers(ctx context.Context) ([]BrokerInfo, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return nil, ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	metadata, err := adminClient.Metadata(ctx)
	if err != nil {
		a.Logger.Error(err, "Failed to get cluster metadata")
		return nil, err
	}

	brokers := make([]BrokerInfo, 0, len(metadata.Brokers))
	for _, b := range metadata.Brokers {
		brokers = append(brokers, BrokerInfo{
			ID:   b.NodeID,
			Host: b.Host,
			Port: b.Port,
			Rack: b.Rack,
		})
	}
	return brokers, nil
}

// ListGroups lists all consumer groups
func (a *Admin) ListGroups(ctx context.Context) ([]string, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return nil, ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	groups, err := adminClient.ListGroups(ctx)
	if err != nil {
		a.Logger.Error(err, "Failed to list consumer groups")
		return nil, err
	}

	result := make([]string, 0, len(groups))
	for _, g := range groups {
		result = append(result, g.Group)
	}
	return result, nil
}

// DescribeGroups returns detailed information about specific consumer groups
func (a *Admin) DescribeGroups(ctx context.Context, groups ...string) ([]GroupInfo, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return nil, ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	described, err := adminClient.DescribeGroups(ctx, groups...)
	if err != nil {
		return nil, err
	}

	result := make([]GroupInfo, 0, len(described))
	for _, g := range described {
		if g.Err != nil {
			continue
		}
		info := GroupInfo{
			Name:         g.Group,
			State:        g.State,
			ProtocolType: g.ProtocolType,
			Protocol:     g.Protocol,
		}
		for _, m := range g.Members {
			info.Members = append(info.Members, GroupMember{
				ID:         m.MemberID,
				ClientID:   m.ClientID,
				ClientHost: m.ClientHost,
			})
		}
		result = append(result, info)
	}
	return result, nil
}

// DeleteGroups deletes consumer groups
func (a *Admin) DeleteGroups(ctx context.Context, groups ...string) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ErrClientClosed
	}
	adminClient := a.adminClient
	a.mu.RUnlock()

	a.Logger.Info("Deleting consumer groups", log.KV{"groups": groups})

	resp, err := adminClient.DeleteGroups(ctx, groups...)
	if err != nil {
		return err
	}
	for _, g := range resp {
		if g.Err != nil {
			a.Logger.Error(g.Err, "Failed to delete group", log.KV{"group": g.Group})
			return g.Err
		}
	}
	return nil
}

// Close closes the admin client
func (a *Admin) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}

	a.closed = true
	a.adminClient = nil
	a.client.Close()
	a.Logger.Info("Admin client closed")
}

// IsConnected returns true if the admin client is connected
func (a *Admin) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return !a.closed && a.adminClient != nil
}

// Client returns the underlying kgo.Client for advanced use cases
func (a *Admin) Client() *kgo.Client {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.client
}

// AdminClient returns the underlying kadm.Client for advanced use cases
func (a *Admin) AdminClient() *kadm.Client {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.adminClient
}
