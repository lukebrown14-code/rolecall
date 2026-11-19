# Projects

## queue-lens

A small open-source terminal tool, written in Go, that shows the depth and oldest message age of
each SQS queue in an AWS account. Built to answer "is the backlog growing?" during incidents.

- Go, AWS SDK, Bubble Tea
- Used by the Harbourline on-call rotation since 2024

## tram-times

A Python service that fetches public transport timetables once an hour and serves them from a
cache to a hobby phone app.

- Python, FastAPI, PostgreSQL
- Runs in Docker on a single small server
