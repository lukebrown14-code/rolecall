# Casey Marlow

Quandong Travel — Platform Engineer

Dear Quandong Travel team,

From the ad, the platform work here looks like steady stewardship: a set of services behind one booking site,
and a team that needs its infrastructure and pipelines to be dependable rather than clever.

Most of my recent work fits that description. At Harbourline Logistics I moved 30 services from
hand-built servers to Docker and Terraform on AWS. The goal was that any engineer could read
how a service was deployed and change it safely, and the Terraform modules we ended up with
did that.

On pipelines, I rebuilt our CI/CD pipeline in GitHub Actions and cut the deploy time from 45
minutes to 8 minutes. Faster deploys meant smaller changes and fewer rollbacks, which matters
for a booking site where a bad release costs sales during the evening peak.

I also share the on-call load you describe. I led the on-call rotation for a team of five and
wrote its runbooks, and I added Prometheus and Grafana monitoring that halved the time it took us
to find the cause of an incident.

The first thing I would look at is the pipelines, measuring each stage of a typical run and
taking out the slowest step before anything else.

One practical note: I hold full working rights in Australia and can start on four weeks' notice.

Kind regards,
Casey Marlow
