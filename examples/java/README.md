# go-hollow Java Examples

This directory contains Java implementations of the three realistic scenarios described in [USAGE.md](../../USAGE.md).

## Prerequisites

- Java 17 or later
- Maven 3.8+ or Gradle 7+
- Generated Cap'n Proto bindings (run `../../tools/gen-schema.sh java`)

## Tech Stack

- **Framework**: Quarkus (for fast startup and low memory footprint)
- **Dependencies**: capnproto-java for Cap'n Proto bindings
- **Storage**: AWS SDK v2 for S3 integration
- **Testing**: Quarkus dev-services for local MinIO

## Installation

```bash
# Create Quarkus project (if not already created)
mvn io.quarkus:quarkus-maven-plugin:create \
    -DprojectGroupId=com.example \
    -DprojectArtifactId=go-hollow-java \
    -DclassName="com.example.MovieResource"

# Install dependencies
mvn quarkus:dev
```

## Running Examples

### Scenario A: Movie Catalog

```bash
# Generate schemas first
cd ../../
./tools/gen-schema.sh java

# Run movie catalog example
cd examples/java
mvn quarkus:dev
# OR
./mvnw compile exec:java -Dexec.mainClass="com.example.movie.MovieMain"
```

### Scenario B: Commerce Orders

```bash
./mvnw compile exec:java -Dexec.mainClass="com.example.commerce.CommerceMain"
```

### Scenario C: IoT Metrics

```bash
./mvnw compile exec:java -Dexec.mainClass="com.example.iot.IoTMain"
```

## Example Structure

Each scenario demonstrates:

1. **Schema Loading**: Using capnproto-java with generated bindings
2. **Producer Setup**: Java wrapper for go-hollow producer
3. **Consumer Setup**: Reactive streams for data consumption
4. **Index Queries**: Java collections integration for indexes
5. **CLI Integration**: ProcessBuilder for hollow-cli integration

## Files

- `src/main/java/com/example/movie/` - Movie catalog scenario
- `src/main/java/com/example/commerce/` - Commerce orders scenario  
- `src/main/java/com/example/iot/` - IoT metrics scenario
- `src/main/java/com/example/hollow/` - Java wrapper library
- `pom.xml` - Maven project configuration
- `application.properties` - Quarkus configuration

## Quarkus Features

- **Dev Services**: Automatic MinIO container for testing
- **Native Compilation**: GraalVM native image support
- **Hot Reload**: Automatic code reloading during development
- **Health Checks**: Built-in health monitoring endpoints

## Configuration

```properties
# application.properties
quarkus.s3.endpoint-override=http://localhost:9000
quarkus.s3.path-style-access=true
quarkus.s3.credentials.type=static
quarkus.s3.credentials.static-provider.access-key-id=minioadmin
quarkus.s3.credentials.static-provider.secret-access-key=minioadmin
```

## TODO (Phase 6)

- Implement capnproto-java schema generation
- Create Java wrapper for go-hollow APIs
- Add reactive streams for announcer
- Implement S3 integration with Quarkus
- Add Quarkus dev-services configuration
- Native image compilation support
