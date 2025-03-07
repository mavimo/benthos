package kafka

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benthosdev/benthos/v4/internal/component/metrics"
	"github.com/benthosdev/benthos/v4/internal/integration"
	"github.com/benthosdev/benthos/v4/internal/log"
	"github.com/benthosdev/benthos/v4/internal/manager/mock"
	"github.com/benthosdev/benthos/v4/internal/message"
	"github.com/benthosdev/benthos/v4/internal/old/output/writer"

	// Bring in legacy definition
	_ "github.com/benthosdev/benthos/v4/internal/interop/legacy"
)

func TestIntegrationSaramaRedpanda(t *testing.T) {
	integration.CheckSkip(t)
	t.Parallel()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	pool.MaxWait = time.Second * 30

	kafkaPort, err := integration.GetFreePort()
	require.NoError(t, err)

	kafkaPortStr := strconv.Itoa(kafkaPort)

	options := &dockertest.RunOptions{
		Repository:   "docker.vectorized.io/vectorized/redpanda",
		Tag:          "latest",
		Hostname:     "redpanda",
		ExposedPorts: []string{"9092"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"9092/tcp": {{HostIP: "", HostPort: kafkaPortStr}},
		},
		Cmd: []string{
			"redpanda", "start", "--smp 1", "--overprovisioned",
			"--kafka-addr 0.0.0.0:9092",
			fmt.Sprintf("--advertise-kafka-addr localhost:%v", kafkaPort),
		},
	}
	resource, err := pool.RunWithOptions(options)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, pool.Purge(resource))
	})

	_ = resource.Expire(900)
	require.NoError(t, pool.Retry(func() error {
		outConf := writer.NewKafkaConfig()
		outConf.TargetVersion = "2.1.0"
		outConf.Addresses = []string{"localhost:" + kafkaPortStr}
		outConf.Topic = "pls_ignore_just_testing_connection"
		tmpOutput, serr := writer.NewKafka(outConf, mock.NewManager(), log.Noop(), metrics.Noop())
		if serr != nil {
			return serr
		}
		defer tmpOutput.CloseAsync()
		if serr := tmpOutput.Connect(); serr != nil {
			return serr
		}
		return tmpOutput.Write(message.QuickBatch([][]byte{
			[]byte("foo message"),
		}))
	}))

	template := `
output:
  kafka:
    addresses: [ localhost:$PORT ]
    topic: topic-$ID
    max_in_flight: $MAX_IN_FLIGHT
    retry_as_batch: $VAR3
    metadata:
      exclude_prefixes: [ $OUTPUT_META_EXCLUDE_PREFIX ]
    batching:
      count: $OUTPUT_BATCH_COUNT

input:
  kafka:
    addresses: [ localhost:$PORT ]
    topics: [ topic-$ID$VAR1 ]
    consumer_group: "$VAR4"
    checkpoint_limit: $VAR2
    start_from_oldest: true
    batching:
      count: $INPUT_BATCH_COUNT
`

	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		integration.StreamTestMetadata(),
		integration.StreamTestMetadataFilter(),
		integration.StreamTestSendBatch(10),
		integration.StreamTestStreamSequential(1000),
		integration.StreamTestStreamParallel(1000),
		integration.StreamTestStreamParallelLossy(1000),
		integration.StreamTestSendBatchCount(10),
	)
	// In some modes include testing input level batching
	var suiteExt integration.StreamTestList
	suiteExt = append(suiteExt, suite...)
	suiteExt = append(suiteExt, integration.StreamTestReceiveBatchCount(10))

	// Only for checkpointed tests
	var suiteSingleCheckpointedStream integration.StreamTestList
	suiteSingleCheckpointedStream = append(suiteSingleCheckpointedStream, suite...)
	suiteSingleCheckpointedStream = append(suiteSingleCheckpointedStream, integration.StreamTestCheckpointCapture())

	t.Run("balanced", func(t *testing.T) {
		t.Parallel()
		suite.Run(
			t, template,
			integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
				vars.Var4 = "group" + testID
				require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 4))
			}),
			integration.StreamTestOptPort(kafkaPortStr),
			integration.StreamTestOptVarTwo("1"),
			integration.StreamTestOptVarThree("false"),
		)

		t.Run("only one partition", func(t *testing.T) {
			t.Parallel()
			suiteExt.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					vars.Var4 = "group" + testID
				}),
				integration.StreamTestOptPort(kafkaPortStr),
				integration.StreamTestOptVarTwo("1"),
				integration.StreamTestOptVarThree("false"),
			)
		})

		t.Run("checkpointed", func(t *testing.T) {
			t.Parallel()
			suite.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					vars.Var4 = "group" + testID
					require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 4))
				}),
				integration.StreamTestOptPort(kafkaPortStr),
				integration.StreamTestOptVarTwo("1000"),
				integration.StreamTestOptVarThree("false"),
			)
		})

		t.Run("retry as batch", func(t *testing.T) {
			t.Parallel()
			suite.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					vars.Var4 = "group" + testID
					require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 4))
				}),
				integration.StreamTestOptPort(kafkaPortStr),
				integration.StreamTestOptVarTwo("1"),
				integration.StreamTestOptVarThree("true"),
			)
		})
	})

	t.Run("explicit partitions", func(t *testing.T) {
		t.Parallel()
		suite.Run(
			t, template,
			integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
				vars.Var4 = "group" + testID
				topicName := "topic-" + testID
				vars.Var1 = fmt.Sprintf(":0,%v:1,%v:2,%v:3", topicName, topicName, topicName)
				require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 4))
			}),
			integration.StreamTestOptPort(kafkaPortStr),
			integration.StreamTestOptSleepAfterInput(time.Second*3),
			integration.StreamTestOptVarTwo("1"),
			integration.StreamTestOptVarThree("false"),
		)

		t.Run("range of partitions", func(t *testing.T) {
			t.Parallel()
			suite.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					vars.Var4 = "group" + testID
					require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 4))
				}),
				integration.StreamTestOptPort(kafkaPortStr),
				integration.StreamTestOptSleepAfterInput(time.Second*3),
				integration.StreamTestOptVarOne(":0-3"),
				integration.StreamTestOptVarTwo("1"),
				integration.StreamTestOptVarThree("false"),
			)
		})

		t.Run("checkpointed", func(t *testing.T) {
			t.Parallel()
			suiteSingleCheckpointedStream.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					vars.Var4 = "group" + testID
					require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 1))
				}),
				integration.StreamTestOptPort(kafkaPortStr),
				integration.StreamTestOptSleepAfterInput(time.Second*3),
				integration.StreamTestOptVarOne(":0"),
				integration.StreamTestOptVarTwo("1000"),
				integration.StreamTestOptVarThree("false"),
			)
		})
	})

	t.Run("without consumer group", func(t *testing.T) {
		t.Parallel()
		suite.Run(
			t, template,
			integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
				require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 4))
			}),
			integration.StreamTestOptPort(kafkaPortStr),
			integration.StreamTestOptSleepAfterInput(time.Second*3),
			integration.StreamTestOptVarOne(":0-3"),
			integration.StreamTestOptVarTwo("1"),
			integration.StreamTestOptVarThree("false"),
		)
	})

	templateManualPartitioner := `
output:
  kafka:
    addresses: [ localhost:$PORT ]
    topic: topic-$ID
    max_in_flight: $MAX_IN_FLIGHT
    retry_as_batch: $VAR3
    metadata:
      exclude_prefixes: [ $OUTPUT_META_EXCLUDE_PREFIX ]
    batching:
      count: $OUTPUT_BATCH_COUNT
    partitioner: manual
    partition: '${! random_int() % 4 }'

input:
  kafka:
    addresses: [ localhost:$PORT ]
    topics: [ topic-$ID$VAR1 ]
    consumer_group: "$VAR4"
    checkpoint_limit: $VAR2
    start_from_oldest: true
    batching:
      count: $INPUT_BATCH_COUNT
`

	t.Run("manual_partitioner", func(t *testing.T) {
		t.Parallel()
		suite.Run(
			t, templateManualPartitioner,
			integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
				vars.Var4 = "group" + testID
				require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 4))
			}),
			integration.StreamTestOptPort(kafkaPortStr),
			integration.StreamTestOptVarTwo("1"),
			integration.StreamTestOptVarThree("false"),
		)
	})

}

func createKafkaTopic(address, id string, partitions int32) error {
	topicName := fmt.Sprintf("topic-%v", id)

	b := sarama.NewBroker(address)
	defer b.Close()

	if err := b.Open(sarama.NewConfig()); err != nil {
		return err
	}

	req := &sarama.CreateTopicsRequest{
		TopicDetails: map[string]*sarama.TopicDetail{
			topicName: {
				NumPartitions:     partitions,
				ReplicationFactor: 1,
			},
		},
	}

	res, err := b.CreateTopics(req)
	if err != nil {
		return err
	}
	if len(res.TopicErrors) > 0 {
		if errStr := res.TopicErrors[topicName].ErrMsg; errStr != nil {
			return errors.New(*errStr)
		}
	}

	var meta *sarama.MetadataResponse
	for i := 0; i < 20; i++ {
		meta, err = b.GetMetadata(&sarama.MetadataRequest{
			Topics: []string{topicName},
		})
		if err == nil && len(meta.Topics) == 1 && len(meta.Topics[0].Partitions) == int(partitions) {
			break
		}
		<-time.After(time.Millisecond * 100)
	}
	if err != nil {
		return err
	}
	if len(meta.Topics) == 0 || len(meta.Topics[0].Partitions) != int(partitions) {
		return fmt.Errorf("failed to create topic: %v", topicName)
	}

	return nil
}

func TestIntegrationSaramaOld(t *testing.T) {
	integration.CheckSkip(t)
	if runtime.GOOS == "darwin" {
		t.Skip("skipping test on macos")
	}

	t.Parallel()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	pool.MaxWait = time.Minute

	networks, _ := pool.Client.ListNetworks()
	hostIP := ""
	for _, network := range networks {
		if network.Name == "bridge" {
			hostIP = network.IPAM.Config[0].Gateway
		}
	}

	zkResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "wurstmeister/zookeeper",
		Tag:        "latest",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pool.Purge(zkResource))
	})
	_ = zkResource.Expire(900)
	zkAddr := fmt.Sprintf("%v:2181", zkResource.Container.NetworkSettings.IPAddress)

	kafkaPort, err := integration.GetFreePort()
	require.NoError(t, err)

	kafkaPortStr := strconv.Itoa(kafkaPort)
	env := []string{
		"KAFKA_ADVERTISED_HOST_NAME=" + hostIP,
		"KAFKA_BROKER_ID=1",
		"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=OUTSIDE:PLAINTEXT,INSIDE:PLAINTEXT",
		"KAFKA_LISTENERS=OUTSIDE://:" + kafkaPortStr + ",INSIDE://:9092",
		"KAFKA_ADVERTISED_LISTENERS=OUTSIDE://" + hostIP + ":" + kafkaPortStr + ",INSIDE://:9092",
		"KAFKA_INTER_BROKER_LISTENER_NAME=INSIDE",
		"KAFKA_ZOOKEEPER_CONNECT=" + zkAddr,
	}

	kafkaResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   "wurstmeister/kafka",
		Tag:          "latest",
		ExposedPorts: []string{kafkaPortStr + "/tcp"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			docker.Port(kafkaPortStr + "/tcp"): {{HostIP: "", HostPort: kafkaPortStr}},
		},
		Env: env,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pool.Purge(kafkaResource))
	})
	_ = kafkaResource.Expire(900)

	address := fmt.Sprintf("%v:%v", hostIP, kafkaPortStr)

	require.NoError(t, pool.Retry(func() error {
		outConf := writer.NewKafkaConfig()
		outConf.TargetVersion = "2.1.0"
		outConf.Addresses = []string{address}
		outConf.Topic = "pls_ignore_just_testing_connection"
		tmpOutput, serr := writer.NewKafka(outConf, mock.NewManager(), log.Noop(), metrics.Noop())
		if serr != nil {
			return serr
		}
		defer tmpOutput.CloseAsync()
		if serr := tmpOutput.Connect(); serr != nil {
			return serr
		}
		return tmpOutput.Write(message.QuickBatch([][]byte{
			[]byte("foo message"),
		}))
	}))

	template := fmt.Sprintf(`
output:
  kafka:
    addresses: [ %v ]
    topic: topic-$ID
    max_in_flight: $MAX_IN_FLIGHT
    retry_as_batch: $VAR3
    batching:
      count: $OUTPUT_BATCH_COUNT

input:
  kafka:
    addresses: [ %v ]
    topics: [ topic-$ID$VAR1 ]
    consumer_group: consumer-$ID
    checkpoint_limit: $VAR2
    batching:
      count: $INPUT_BATCH_COUNT
`, address, address)

	suite := integration.StreamTests(
		integration.StreamTestOpenClose(),
		integration.StreamTestMetadata(),
		integration.StreamTestSendBatch(10),
		integration.StreamTestStreamSequential(1000),
		integration.StreamTestStreamParallel(1000),
		integration.StreamTestStreamParallelLossy(1000),
		integration.StreamTestSendBatchCount(10),
	)
	// In some tests include testing input level batching
	var suiteExt integration.StreamTestList
	suiteExt = append(suiteExt, suite...)
	suiteExt = append(suiteExt, integration.StreamTestReceiveBatchCount(10))

	// Only for checkpointed tests
	var suiteSingleCheckpointedStream integration.StreamTestList
	suiteSingleCheckpointedStream = append(suiteSingleCheckpointedStream, suite...)
	suiteSingleCheckpointedStream = append(suiteSingleCheckpointedStream, integration.StreamTestCheckpointCapture())

	t.Run("balanced", func(t *testing.T) {
		t.Parallel()
		suiteExt.Run(
			t, template,
			integration.StreamTestOptVarOne(""),
			integration.StreamTestOptVarTwo("1"),
			integration.StreamTestOptVarThree("false"),
		)

		t.Run("checkpointed", func(t *testing.T) {
			t.Parallel()
			suiteSingleCheckpointedStream.Run(
				t, template,
				integration.StreamTestOptVarOne(""),
				integration.StreamTestOptVarTwo("1000"),
				integration.StreamTestOptVarThree("false"),
			)
		})

		t.Run("retry as batch", func(t *testing.T) {
			t.Parallel()
			suiteExt.Run(
				t, template,
				integration.StreamTestOptVarOne(""),
				integration.StreamTestOptVarTwo("1"),
				integration.StreamTestOptVarThree("true"),
			)
		})

		t.Run("with four partitions", func(t *testing.T) {
			t.Parallel()
			suite.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					require.NoError(t, createKafkaTopic(address, testID, 4))
				}),
				integration.StreamTestOptVarOne(""),
				integration.StreamTestOptVarTwo("1"),
				integration.StreamTestOptVarThree("false"),
			)

			t.Run("checkpointed", func(t *testing.T) {
				t.Parallel()
				suite.Run(
					t, template,
					integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
						require.NoError(t, createKafkaTopic(address, testID, 4))
					}),
					integration.StreamTestOptVarOne(""),
					integration.StreamTestOptVarTwo("1000"),
					integration.StreamTestOptVarThree("false"),
				)
			})
		})
	})

	t.Run("partitions", func(t *testing.T) {
		t.Parallel()
		suiteExt.Run(
			t, template,
			integration.StreamTestOptVarOne(":0"),
			integration.StreamTestOptVarTwo("1"),
			integration.StreamTestOptVarThree("false"),
		)

		t.Run("checkpointed", func(t *testing.T) {
			t.Parallel()
			suiteSingleCheckpointedStream.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					require.NoError(t, createKafkaTopic("localhost:"+kafkaPortStr, testID, 1))
				}),
				integration.StreamTestOptVarOne(":0"),
				integration.StreamTestOptVarTwo("1000"),
				integration.StreamTestOptVarThree("false"),
			)
		})

		t.Run("with four partitions", func(t *testing.T) {
			t.Parallel()
			suite.Run(
				t, template,
				integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
					topicName := "topic-" + testID
					vars.Var1 = fmt.Sprintf(":0,%v:1,%v:2,%v:3", topicName, topicName, topicName)
					require.NoError(t, createKafkaTopic(address, testID, 4))
				}),
				integration.StreamTestOptSleepAfterInput(time.Second*3),
				integration.StreamTestOptVarTwo("1"),
				integration.StreamTestOptVarThree("false"),
			)

			t.Run("checkpointed", func(t *testing.T) {
				t.Parallel()
				suite.Run(
					t, template,
					integration.StreamTestOptPreTest(func(t testing.TB, ctx context.Context, testID string, vars *integration.StreamTestConfigVars) {
						topicName := "topic-" + testID
						vars.Var1 = fmt.Sprintf(":0,%v:1,%v:2,%v:3", topicName, topicName, topicName)
						require.NoError(t, createKafkaTopic(address, testID, 4))
					}),
					integration.StreamTestOptSleepAfterInput(time.Second*3),
					integration.StreamTestOptVarTwo("1000"),
					integration.StreamTestOptVarThree("false"),
				)
			})
		})
	})
}
