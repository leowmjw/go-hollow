# Java Bindings

Java bindings will be generated here using capnproto-java.

## Installation
Add to pom.xml:
```xml
<dependency>
    <groupId>org.capnproto</groupId>
    <artifactId>runtime</artifactId>
    <version>0.1.14</version>
</dependency>
```

## Generation (TODO)
```bash
capnp compile -I../../schemas -ojava:. ../../schemas/*.capnp
```
