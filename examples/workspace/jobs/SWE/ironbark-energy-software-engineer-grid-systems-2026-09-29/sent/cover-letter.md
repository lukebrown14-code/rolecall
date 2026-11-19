# Casey Marlow

Ironbark Energy — Software Engineer, Grid Systems

Dear Ironbark Energy team,

From the ad, the core problem for Grid Systems looks like turning a steady stream of battery
telemetry into decisions quickly enough to matter, and knowing straight away when that stream
goes quiet.

The shape of that work is familiar. At Harbourline Logistics I built and ran the shipment
tracking API in Go and PostgreSQL, taking updates from 40 warehouse sites and serving them back
to the people who needed them. The parts that took the most care were the same ones your ad
points at: late or missing data, and queries that stay fast as the tables grow.

Knowing when something has gone quiet is an observability question. I added Prometheus and
Grafana monitoring to that platform, which halved the time it took us to find the cause of an
incident, and I led the on-call rotation for a team of five and wrote its runbooks.

I have used PostgreSQL for time-ordered data but not a dedicated time-series database, and energy
markets are new to me. I would expect to spend the first weeks learning how dispatch decisions
are made before changing the services that make them.

The place I would be useful first is alerting on missing telemetry, so a silent battery is
noticed in minutes rather than at the end of the day.

One practical note: I hold full working rights in Australia and can start on four weeks' notice.

Kind regards,
Casey Marlow
