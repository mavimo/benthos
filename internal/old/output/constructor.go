package output

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/benthosdev/benthos/v4/internal/component"
	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/component/output"
	iprocessor "github.com/benthosdev/benthos/v4/internal/component/processor"
	"github.com/benthosdev/benthos/v4/internal/docs"
	"github.com/benthosdev/benthos/v4/internal/interop"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/old/output/writer"
	"github.com/benthosdev/benthos/v4/internal/old/processor"
	"github.com/benthosdev/benthos/v4/internal/pipeline"
)

// TypeSpec is a constructor and a usage description for each output type.
type TypeSpec struct {
	constructor ConstructorFunc

	// Async indicates whether this output benefits from sending multiple
	// messages asynchronously over the protocol.
	Async bool

	// Batches indicates whether this output benefits from batching of messages.
	Batches bool

	Status      docs.Status
	Summary     string
	Description string
	Categories  []string
	Footnotes   string
	Config      docs.FieldSpec
	Examples    []docs.AnnotatedExample
	Version     string
}

// AppendProcessorsFromConfig takes a variant arg of pipeline constructor
// functions and returns a new slice of them where the processors of the
// provided output configuration will also be initialized.
func AppendProcessorsFromConfig(conf Config, mgr interop.Manager, pipelines ...iprocessor.PipelineConstructorFunc) []iprocessor.PipelineConstructorFunc {
	if len(conf.Processors) > 0 {
		pipelines = append(pipelines, []iprocessor.PipelineConstructorFunc{func() (iprocessor.Pipeline, error) {
			processors := make([]iprocessor.V1, len(conf.Processors))
			for j, procConf := range conf.Processors {
				var err error
				pMgr := mgr.IntoPath("processors", strconv.Itoa(j))
				processors[j], err = processor.New(procConf, pMgr, pMgr.Logger(), pMgr.Metrics())
				if err != nil {
					return nil, err
				}
			}
			return pipeline.NewProcessor(processors...), nil
		}}...)
	}
	return pipelines
}

func fromSimpleConstructor(fn func(Config, interop.Manager, log.Modular, metrics.Type) (output.Streamed, error)) ConstructorFunc {
	return func(
		conf Config,
		mgr interop.Manager,
		log log.Modular,
		stats metrics.Type,
		pipelines ...iprocessor.PipelineConstructorFunc,
	) (output.Streamed, error) {
		output, err := fn(conf, mgr, log, stats)
		if err != nil {
			return nil, err
		}
		pipelines = AppendProcessorsFromConfig(conf, mgr, pipelines...)
		return WrapWithPipelines(output, pipelines...)
	}
}

// ConstructorFunc is a func signature able to construct an output.
type ConstructorFunc func(Config, interop.Manager, log.Modular, metrics.Type, ...iprocessor.PipelineConstructorFunc) (output.Streamed, error)

// WalkConstructors iterates each component constructor.
func WalkConstructors(fn func(ConstructorFunc, docs.ComponentSpec)) {
	inferred := docs.ComponentFieldsFromConf(NewConfig())
	for k, v := range Constructors {
		conf := v.Config
		conf.Children = conf.Children.DefaultAndTypeFrom(inferred[k])
		spec := docs.ComponentSpec{
			Type:        docs.TypeOutput,
			Name:        k,
			Summary:     v.Summary,
			Description: v.Description,
			Footnotes:   v.Footnotes,
			Categories:  v.Categories,
			Config:      conf,
			Examples:    v.Examples,
			Status:      v.Status,
			Version:     v.Version,
		}
		spec.Description = output.Description(v.Async, v.Batches, spec.Description)
		fn(v.constructor, spec)
	}
}

// Constructors is a map of all output types with their specs.
var Constructors = map[string]TypeSpec{}

//------------------------------------------------------------------------------

// String constants representing each output type.
// Deprecated: Do not add new components here. Instead, use the public plugin
// APIs. Examples can be found in: ./internal/impl
const (
	TypeAMQP09             = "amqp_0_9"
	TypeAMQP1              = "amqp_1"
	TypeAWSDynamoDB        = "aws_dynamodb"
	TypeAWSKinesis         = "aws_kinesis"
	TypeAWSKinesisFirehose = "aws_kinesis_firehose"
	TypeAWSS3              = "aws_s3"
	TypeAWSSNS             = "aws_sns"
	TypeAWSSQS             = "aws_sqs"
	TypeAzureBlobStorage   = "azure_blob_storage"
	TypeAzureQueueStorage  = "azure_queue_storage"
	TypeAzureTableStorage  = "azure_table_storage"
	TypeBroker             = "broker"
	TypeCache              = "cache"
	TypeCassandra          = "cassandra"
	TypeDrop               = "drop"
	TypeDropOn             = "drop_on"
	TypeDynamic            = "dynamic"
	TypeDynamoDB           = "dynamodb"
	TypeElasticsearch      = "elasticsearch"
	TypeFallback           = "fallback"
	TypeFile               = "file"
	TypeGCPCloudStorage    = "gcp_cloud_storage"
	TypeGCPPubSub          = "gcp_pubsub"
	TypeHDFS               = "hdfs"
	TypeHTTPClient         = "http_client"
	TypeHTTPServer         = "http_server"
	TypeInproc             = "inproc"
	TypeKafka              = "kafka"
	TypeMongoDB            = "mongodb"
	TypeMQTT               = "mqtt"
	TypeNanomsg            = "nanomsg"
	TypeNATS               = "nats"
	TypeNATSJetStream      = "nats_jetstream"
	TypeNATSStream         = "nats_stream"
	TypeNSQ                = "nsq"
	TypeRedisHash          = "redis_hash"
	TypeRedisList          = "redis_list"
	TypeRedisPubSub        = "redis_pubsub"
	TypeRedisStreams       = "redis_streams"
	TypeReject             = "reject"
	TypeResource           = "resource"
	TypeRetry              = "retry"
	TypeSFTP               = "sftp"
	TypeSTDOUT             = "stdout"
	TypeSubprocess         = "subprocess"
	TypeSwitch             = "switch"
	TypeSyncResponse       = "sync_response"
	TypeSocket             = "socket"
	TypeWebsocket          = "websocket"
)

//------------------------------------------------------------------------------

// Config is the all encompassing configuration struct for all output types.
// Deprecated: Do not add new components here. Instead, use the public plugin
// APIs. Examples can be found in: ./internal/impl
type Config struct {
	Label              string                         `json:"label" yaml:"label"`
	Type               string                         `json:"type" yaml:"type"`
	AMQP09             AMQPConfig                     `json:"amqp_0_9" yaml:"amqp_0_9"`
	AMQP1              AMQP1Config                    `json:"amqp_1" yaml:"amqp_1"`
	AWSDynamoDB        DynamoDBConfig                 `json:"aws_dynamodb" yaml:"aws_dynamodb"`
	AWSKinesis         KinesisConfig                  `json:"aws_kinesis" yaml:"aws_kinesis"`
	AWSKinesisFirehose KinesisFirehoseConfig          `json:"aws_kinesis_firehose" yaml:"aws_kinesis_firehose"`
	AWSS3              AmazonS3Config                 `json:"aws_s3" yaml:"aws_s3"`
	AWSSNS             SNSConfig                      `json:"aws_sns" yaml:"aws_sns"`
	AWSSQS             AmazonSQSConfig                `json:"aws_sqs" yaml:"aws_sqs"`
	AzureBlobStorage   writer.AzureBlobStorageConfig  `json:"azure_blob_storage" yaml:"azure_blob_storage"`
	AzureQueueStorage  writer.AzureQueueStorageConfig `json:"azure_queue_storage" yaml:"azure_queue_storage"`
	AzureTableStorage  writer.AzureTableStorageConfig `json:"azure_table_storage" yaml:"azure_table_storage"`
	Broker             BrokerConfig                   `json:"broker" yaml:"broker"`
	Cache              writer.CacheConfig             `json:"cache" yaml:"cache"`
	Cassandra          CassandraConfig                `json:"cassandra" yaml:"cassandra"`
	Drop               DropConfig                     `json:"drop" yaml:"drop"`
	DropOn             DropOnConfig                   `json:"drop_on" yaml:"drop_on"`
	Dynamic            DynamicConfig                  `json:"dynamic" yaml:"dynamic"`
	Elasticsearch      writer.ElasticsearchConfig     `json:"elasticsearch" yaml:"elasticsearch"`
	Fallback           TryConfig                      `json:"fallback" yaml:"fallback"`
	File               FileConfig                     `json:"file" yaml:"file"`
	GCPCloudStorage    GCPCloudStorageConfig          `json:"gcp_cloud_storage" yaml:"gcp_cloud_storage"`
	GCPPubSub          writer.GCPPubSubConfig         `json:"gcp_pubsub" yaml:"gcp_pubsub"`
	HDFS               writer.HDFSConfig              `json:"hdfs" yaml:"hdfs"`
	HTTPClient         writer.HTTPClientConfig        `json:"http_client" yaml:"http_client"`
	HTTPServer         HTTPServerConfig               `json:"http_server" yaml:"http_server"`
	Inproc             InprocConfig                   `json:"inproc" yaml:"inproc"`
	Kafka              writer.KafkaConfig             `json:"kafka" yaml:"kafka"`
	MongoDB            MongoDBConfig                  `json:"mongodb" yaml:"mongodb"`
	MQTT               MQTTConfig                     `json:"mqtt" yaml:"mqtt"`
	Nanomsg            writer.NanomsgConfig           `json:"nanomsg" yaml:"nanomsg"`
	NATS               writer.NATSConfig              `json:"nats" yaml:"nats"`
	NATSStream         writer.NATSStreamConfig        `json:"nats_stream" yaml:"nats_stream"`
	NSQ                writer.NSQConfig               `json:"nsq" yaml:"nsq"`
	Plugin             interface{}                    `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	RedisHash          writer.RedisHashConfig         `json:"redis_hash" yaml:"redis_hash"`
	RedisList          writer.RedisListConfig         `json:"redis_list" yaml:"redis_list"`
	RedisPubSub        writer.RedisPubSubConfig       `json:"redis_pubsub" yaml:"redis_pubsub"`
	RedisStreams       writer.RedisStreamsConfig      `json:"redis_streams" yaml:"redis_streams"`
	Reject             RejectConfig                   `json:"reject" yaml:"reject"`
	Resource           string                         `json:"resource" yaml:"resource"`
	Retry              RetryConfig                    `json:"retry" yaml:"retry"`
	SFTP               SFTPConfig                     `json:"sftp" yaml:"sftp"`
	STDOUT             STDOUTConfig                   `json:"stdout" yaml:"stdout"`
	Subprocess         SubprocessConfig               `json:"subprocess" yaml:"subprocess"`
	Switch             SwitchConfig                   `json:"switch" yaml:"switch"`
	SyncResponse       struct{}                       `json:"sync_response" yaml:"sync_response"`
	Socket             writer.SocketConfig            `json:"socket" yaml:"socket"`
	Websocket          writer.WebsocketConfig         `json:"websocket" yaml:"websocket"`
	Processors         []processor.Config             `json:"processors" yaml:"processors"`
}

// NewConfig returns a configuration struct fully populated with default values.
// Deprecated: Do not add new components here. Instead, use the public plugin
// APIs. Examples can be found in: ./internal/impl
func NewConfig() Config {
	return Config{
		Label:              "",
		Type:               "stdout",
		AMQP09:             NewAMQPConfig(),
		AMQP1:              NewAMQP1Config(),
		AWSDynamoDB:        NewDynamoDBConfig(),
		AWSKinesis:         NewKinesisConfig(),
		AWSKinesisFirehose: NewKinesisFirehoseConfig(),
		AWSS3:              NewAmazonS3Config(),
		AWSSNS:             NewSNSConfig(),
		AWSSQS:             NewAmazonSQSConfig(),
		AzureBlobStorage:   writer.NewAzureBlobStorageConfig(),
		AzureQueueStorage:  writer.NewAzureQueueStorageConfig(),
		AzureTableStorage:  writer.NewAzureTableStorageConfig(),
		Broker:             NewBrokerConfig(),
		Cache:              writer.NewCacheConfig(),
		Cassandra:          NewCassandraConfig(),
		Drop:               NewDropConfig(),
		DropOn:             NewDropOnConfig(),
		Dynamic:            NewDynamicConfig(),
		Elasticsearch:      writer.NewElasticsearchConfig(),
		Fallback:           NewTryConfig(),
		File:               NewFileConfig(),
		GCPCloudStorage:    NewGCPCloudStorageConfig(),
		GCPPubSub:          writer.NewGCPPubSubConfig(),
		HDFS:               writer.NewHDFSConfig(),
		HTTPClient:         writer.NewHTTPClientConfig(),
		HTTPServer:         NewHTTPServerConfig(),
		Inproc:             NewInprocConfig(),
		Kafka:              writer.NewKafkaConfig(),
		MQTT:               NewMQTTConfig(),
		MongoDB:            NewMongoDBConfig(),
		Nanomsg:            writer.NewNanomsgConfig(),
		NATS:               writer.NewNATSConfig(),
		NATSStream:         writer.NewNATSStreamConfig(),
		NSQ:                writer.NewNSQConfig(),
		Plugin:             nil,
		RedisHash:          writer.NewRedisHashConfig(),
		RedisList:          writer.NewRedisListConfig(),
		RedisPubSub:        writer.NewRedisPubSubConfig(),
		RedisStreams:       writer.NewRedisStreamsConfig(),
		Reject:             NewRejectConfig(),
		Resource:           "",
		Retry:              NewRetryConfig(),
		SFTP:               NewSFTPConfig(),
		STDOUT:             NewSTDOUTConfig(),
		Subprocess:         NewSubprocessConfig(),
		Switch:             NewSwitchConfig(),
		SyncResponse:       struct{}{},
		Socket:             writer.NewSocketConfig(),
		Websocket:          writer.NewWebsocketConfig(),
		Processors:         []processor.Config{},
	}
}

//------------------------------------------------------------------------------

// UnmarshalYAML ensures that when parsing configs that are in a map or slice
// the default values are still applied.
func (conf *Config) UnmarshalYAML(value *yaml.Node) error {
	type confAlias Config
	aliased := confAlias(NewConfig())

	err := value.Decode(&aliased)
	if err != nil {
		return fmt.Errorf("line %v: %v", value.Line, err)
	}

	var spec docs.ComponentSpec
	if aliased.Type, spec, err = docs.GetInferenceCandidateFromYAML(docs.DeprecatedProvider, docs.TypeOutput, value); err != nil {
		return fmt.Errorf("line %v: %w", value.Line, err)
	}

	if spec.Plugin {
		pluginNode, err := docs.GetPluginConfigYAML(aliased.Type, value)
		if err != nil {
			return fmt.Errorf("line %v: %v", value.Line, err)
		}
		aliased.Plugin = &pluginNode
	} else {
		aliased.Plugin = nil
	}

	*conf = Config(aliased)
	return nil
}

//------------------------------------------------------------------------------

// New creates an output type based on an output configuration.
func New(
	conf Config,
	mgr interop.Manager,
	log log.Modular,
	stats metrics.Type,
	pipelines ...iprocessor.PipelineConstructorFunc,
) (output.Streamed, error) {
	if mgrV2, ok := mgr.(interface {
		NewOutput(Config, ...iprocessor.PipelineConstructorFunc) (output.Streamed, error)
	}); ok {
		return mgrV2.NewOutput(conf, pipelines...)
	}
	if c, ok := Constructors[conf.Type]; ok {
		return c.constructor(conf, mgr, log, stats, pipelines...)
	}
	return nil, component.ErrInvalidType("output", conf.Type)
}
