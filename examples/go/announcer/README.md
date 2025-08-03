# Announcer Capabilities Example

This example comprehensively demonstrates all capabilities of the go-hollow Announcer system, including advanced features not covered in other examples.

## What This Example Tests

### 1. Pub/Sub Pattern (`demonstratePubSub`)
- **Multiple subscribers** receiving announcements simultaneously
- **Subscribe/Unsubscribe** mechanics with proper channel management
- **Subscriber counting** and lifecycle management
- **Broadcast delivery** to all active subscribers

### 2. Version Waiting (`demonstrateVersionWaiting`)
- **`WaitForVersion()` with timeout** scenarios
- **Success cases**: Version arrives within timeout
- **Timeout cases**: Version doesn't arrive in time
- **Immediate return**: Requesting already available versions
- **Performance timing** measurements

### 3. Pin/Unpin Mechanics (`demonstratePinUnpin`)
- **Version pinning**: Freezing consumers to specific versions
- **Pinned version priority**: Pinned version returned instead of latest
- **Unpin behavior**: Returning to latest version tracking
- **Subscriber notifications** during pin/unpin operations
- **Real-world maintenance scenarios**

### 4. Multi-Consumer Coordination (`demonstrateMultiConsumerCoordination`)
- **Multiple consumers** with different refresh strategies
- **Coordinated updates**: Consumers syncing to same version
- **Staggered updates**: Different timing patterns
- **Pinning in multi-consumer environment**
- **Producer/consumer integration** with announcements

### 5. High-Frequency Performance (`demonstrateHighFrequency`)
- **Rapid announcement** handling (1000+ announcements)
- **Multiple subscriber** performance under load
- **Delivery success rates** and performance metrics
- **Buffer management** for high-throughput scenarios
- **Performance timing** and rate calculations

### 6. Error Scenarios (`demonstrateErrorScenarios`)
- **Dead subscriber cleanup**: Closed channels automatically removed
- **Full channel handling**: Graceful handling of blocked subscribers
- **Resource cleanup**: Proper cleanup on announcer shutdown
- **Context cancellation**: Graceful cancellation of wait operations
- **Edge case resilience**: System stability under error conditions

### 7. Real Integration (`demonstrateRealIntegration`)
- **Real producer/consumer workflow** with announcements
- **Cap'n Proto data** with actual movie schemas
- **Monitoring subscriptions**: Real-time announcement tracking
- **Emergency maintenance**: Pinning for stability during updates
- **Recovery procedures**: Smooth unpinning and synchronization

## Key Features Demonstrated

### Advanced Announcer Features
- **`GoroutineAnnouncer`**: High-performance goroutine-based implementation
- **`Subscribe()/Unsubscribe()`**: Dynamic subscriber management
- **`WaitForVersion()`**: Blocking wait with timeout
- **`Pin()/Unpin()`**: Version control for maintenance scenarios
- **`GetSubscriberCount()`**: Runtime subscriber tracking

### Real-World Scenarios
- **Emergency maintenance**: Pinning consumers during updates
- **Staggered rollouts**: Different consumer update timing
- **High-frequency updates**: IoT/telemetry-style data streams
- **Multi-consumer coordination**: Different services consuming same data
- **Performance monitoring**: Tracking announcement delivery

### Error Handling
- **Graceful degradation**: System continues despite subscriber failures
- **Resource cleanup**: Automatic cleanup of dead subscribers
- **Timeout handling**: Configurable timeouts for version waiting
- **Context cancellation**: Proper cancellation support

## Running the Example

```bash
# From the project root
cd examples/go/announcer
go run main.go
```

## Expected Output

The example will demonstrate each phase with detailed logging:

1. **Phase 1**: Multiple subscribers receiving the same announcements
2. **Phase 2**: Version waiting with different timeout scenarios  
3. **Phase 3**: Pin/unpin operations affecting version visibility
4. **Phase 4**: Multiple consumers coordinating through announcements
5. **Phase 5**: High-frequency performance test with metrics
6. **Phase 6**: Error scenarios and recovery testing
7. **Phase 7**: Real producer/consumer integration

## Integration with Other Examples

This example complements the other go-hollow examples:

- **Movie Example**: Basic announcer usage
- **Commerce Example**: Multi-producer coordination
- **IoT Example**: High-throughput scenarios  
- **Evolution Example**: Schema changes with announcements
- **Announcer Example**: **Complete announcer testing** (this example)

## Performance Characteristics

The example measures and reports:
- **Announcement rate**: Announcements per second
- **Delivery success rate**: Percentage of successful deliveries
- **Latency**: Time from announcement to delivery
- **Resource usage**: Subscriber count and cleanup efficiency

This makes it valuable for understanding announcer performance in production scenarios.
