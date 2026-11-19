# Casey Marlow

Corella Pay — Software Engineer, Payments Platform

Dear Corella Pay team,

From the ad, the hard part of this role looks less like moving money and more like recovering
quickly when something upstream fails: a bank feed that stalls at settlement time, with
thousands of businesses waiting on the result.

That problem is mostly about visibility. At Harbourline Logistics I added Prometheus and Grafana
monitoring to the shipment tracking platform, which halved the time it took us to find the
cause of an incident. I also led the on-call rotation for a team of five and wrote its runbooks,
so the next person paged had a written path to follow.

The second priority in the ad is getting changes to production safely and often. I rebuilt our
CI/CD pipeline in GitHub Actions and cut the deploy time from 45 minutes to 8 minutes, which
made small, frequent releases the normal way to work rather than the exception.

The services themselves would be familiar ground. I built and ran the shipment tracking API in
Go and PostgreSQL for 40 warehouse sites, and moved 30 services to Docker and Terraform on AWS.
I have not worked in payments, and I would expect to spend my first weeks learning how
authorisation and settlement fit together before changing either.

Where I would be useful first is the settlement pipeline's alerting: making a stalled feed
obvious within minutes, with a runbook beside the alert.

One practical note: I hold full working rights in Australia, so I can start without any
sponsorship.

Kind regards,
Casey Marlow
