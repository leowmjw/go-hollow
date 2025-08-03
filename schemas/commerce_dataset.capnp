@0xb2c3d4e5f6081a2b;

using Go = import "std/go.capnp";
$Go.package("commerce");
$Go.import("github.com/leowmjw/go-hollow/generated/go/commerce");

using Common = import "common.capnp";

# E-commerce platform schema v1
# Demonstrates multi-producer scenarios and type filtering

struct Customer {
  id @0 :UInt32;           # Unique customer ID - used for Primary Key
  email @1 :Text;          # Email address - used for Unique index (must be unique)
  name @2 :Text;           # Full customer name
  city @3 :Text;           # City for shipping - used for Hash index
  age @4 :UInt16;          # Age for analytics - used for Hash index
  registeredAt @5 :UInt64; # Registration timestamp
}

struct Order {
  id @0 :UInt32;           # Unique order ID - part of Primary Key
  customerId @1 :UInt32;   # References Customer.id - part of Primary Key
  amount @2 :UInt64;       # Order total in cents (avoid float precision issues)
  currency @3 :Text;       # ISO 4217 currency code (USD, EUR, etc.)
  status @4 :OrderStatus;  # Order status enum
  timestamp @5 :UInt64;    # Order creation time
  items @6 :List(OrderItem); # Items in this order
}

struct OrderItem {
  productId @0 :UInt32;    # Product identifier
  quantity @1 :UInt32;     # Quantity ordered
  pricePerUnit @2 :UInt64; # Price per unit in cents
  productName @3 :Text;    # Product name (denormalized for convenience)
}

enum OrderStatus {
  pending @0;
  confirmed @1;
  shipped @2;
  delivered @3;
  cancelled @4;
  refunded @5;
}

# Root container for the commerce dataset
struct CommerceDataset {
  customers @0 :List(Customer);
  orders @1 :List(Order);
  version @2 :UInt32;      # Dataset version for tracking
}
