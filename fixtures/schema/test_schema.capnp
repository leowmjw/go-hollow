@0xabcdef1234567890;  # Unique file ID

struct Person {
  id @0 :UInt32;
  name @1 :Text;
  email @2 :Text;
  phone @3 :Text;
  address @4 :Address;
  birthDate @5 :UInt64;  # Unix timestamp
  isActive @6 :Bool;
}

struct Address {
  street @0 :Text;
  city @1 :Text;
  state @2 :Text;
  zipCode @3 :Text;
  country @4 :Text;
}

enum Role {
  user @0;
  admin @1;
  guest @2;
}
