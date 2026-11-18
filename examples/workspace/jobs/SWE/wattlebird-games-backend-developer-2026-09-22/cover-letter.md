# Casey Marlow

Wattlebird Games — Backend Developer

Dear Wattlebird Games team,

Reading the ad, the hard part of this role looks like the weekend: leaderboards and events that
sit quietly all week and then take most of their traffic in a few hours, with players noticing
every slow response.

Handling spikes well starts with seeing them. At Harbourline Logistics I added Prometheus and
Grafana monitoring to our services, which halved the time it took us to find the cause of an
incident, and I led the on-call rotation for a team of five. Most of what we learned went into
runbooks I wrote, so the next spike was handled the same way.

The services themselves are close to what I do now. I built and ran the shipment tracking API
in Go and PostgreSQL for 40 warehouse sites, and moved 30 services to Docker and Terraform on
AWS. I have not used Redis in production, and I would want to learn how you use it for
leaderboards before changing anything that touches it.

Releases matter too when an event has a fixed start time. I rebuilt our CI/CD pipeline in GitHub
Actions and cut the deploy time from 45 minutes to 8 minutes, which made it practical to ship a
fix during a busy window rather than wait.

Where I would start is load: replaying a past event against the leaderboard service to find the
first thing that slows down.

One practical note: I hold full working rights in Australia and can start on four weeks' notice.

Kind regards,
Casey Marlow
