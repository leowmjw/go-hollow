@0xc3d4e5f6081a2b3c;

using Go = import "std/go.capnp";
$Go.package("iot");
$Go.import("github.com/leowmjw/go-hollow/generated/go/iot");

using Common = import "common.capnp";

# IoT platform schema v1
# Demonstrates high-throughput scenarios and memory management

struct Device {
  id @0 :UInt32;           # Device ID - used for Hash index
  serial @1 :Text;         # Serial number - used for Unique index (must be unique)
  type @2 :DeviceType;     # Device type enum
  location @3 :Text;       # Physical location description
  manufacturer @4 :Text;   # Device manufacturer
  model @5 :Text;          # Device model
  installedAt @6 :UInt64;  # Installation timestamp
  lastSeenAt @7 :UInt64;   # Last communication timestamp
}

struct Metric {
  deviceId @0 :UInt32;     # References Device.id - part of Hash index
  type @1 :MetricType;     # Metric type enum - part of Hash index
  value @2 :Float64;       # Measured value
  timestamp @3 :UInt64;    # Measurement time - part of Primary Key
  quality @4 :UInt8;       # Data quality score 0-100
  unit @5 :Text;           # Measurement unit (°C, %, kPa, etc.)
}

enum DeviceType {
  sensor @0;
  actuator @1;
  gateway @2;
  controller @3;
  display @4;
}

enum MetricType {
  temperature @0;
  humidity @1;
  pressure @2;
  voltage @3;
  current @4;
  power @5;
  vibration @6;
  motion @7;
  light @8;
  sound @9;
}

# Alert generated when metrics exceed thresholds
struct Alert {
  id @0 :UInt32;           # Alert ID
  deviceId @1 :UInt32;     # Device that triggered alert
  metricType @2 :MetricType; # Type of metric that triggered alert
  severity @3 :AlertSeverity; # Alert severity level
  message @4 :Text;        # Human-readable alert message
  triggeredAt @5 :UInt64;  # When alert was triggered
  resolvedAt @6 :UInt64;   # When alert was resolved (0 if still active)
}

enum AlertSeverity {
  info @0;
  warning @1;
  error @2;
  critical @3;
}

# Root container for the IoT dataset
struct IoTDataset {
  devices @0 :List(Device);
  metrics @1 :List(Metric);
  alerts @2 :List(Alert);
  version @3 :UInt32;      # Dataset version for tracking
}
