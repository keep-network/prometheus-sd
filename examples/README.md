# Keep Network Nodes Discovery Example

To build the docker image:
```
docker compose build
```

To start the example run:
```
docker compose up
```

Open Prometheus to verify the nodes are discovered:
http://localhost:9090/targets

Open Grafana to visualize metrics:
http://localhost:3000/
