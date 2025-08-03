@0xa1b2c3d4e5f60819;

using Go = import "std/go.capnp";
$Go.package("movie");
$Go.import("github.com/leowmjw/go-hollow/generated/go/movie");

using Common = import "common.capnp";

# Movie catalog schema v1
# Schema evolution: v2 will add runtimeMin field to Movie

struct Movie {
  id @0 :UInt32;           # Unique identifier - used for Unique index
  title @1 :Text;          # Movie title
  year @2 :UInt16;         # Release year - used for Hash index
  genres @3 :List(Text);   # Genre tags - used for Hash index
  runtimeMin @4 :UInt16;   # Runtime in minutes (ADDED in v2 for schema evolution demo)
}

struct Rating {
  movieId @0 :UInt32;      # References Movie.id - part of Primary Key
  userId @1 :UInt32;       # User identifier - part of Primary Key  
  score @2 :Float32;       # Rating 1.0-5.0
  timestamp @3 :Common.Timestamp;  # When rating was given (using common timestamp type)
}

# Root container for the movie dataset
struct MovieDataset {
  movies @0 :List(Movie);
  ratings @1 :List(Rating);
  version @2 :UInt32;      # Dataset version for tracking
}
