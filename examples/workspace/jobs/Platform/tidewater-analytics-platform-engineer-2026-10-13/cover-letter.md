# Casey Marlow

Tidewater Analytics — Platform Engineer

Dear Tidewater Analytics team,

From the ad, the platform team's job looks like keeping 40 engineers moving: pipelines they
trust, infrastructure they can change without asking, and alerts that point at the real
problem.

The pipeline is where I would start. At Harbourline Logistics I rebuilt our CI/CD pipeline in
GitHub Actions and cut the deploy time from 45 minutes to 8 minutes. The change that mattered
most was not speed alone: once a deploy took minutes, engineers shipped smaller changes and
rolled back less.

On infrastructure, I moved 30 services from hand-built servers to Docker and Terraform on AWS.
That work gave the team shared Terraform modules instead of one-off scripts, which is close to
the module ownership the ad describes. I have used AWS heavily but have not designed a
multi-account structure from scratch, and I would want to understand yours before proposing
changes to it.

For observability, I added Prometheus and Grafana monitoring that halved the time it took us to
find the cause of an incident, and I led the on-call rotation for a team of five and wrote its
runbooks.

Early on I would be most useful on the pipelines: measuring where time goes in a typical run and
removing the slowest step first.

One practical note: I hold full working rights in Australia and can start on four weeks' notice.

Kind regards,
Casey Marlow
