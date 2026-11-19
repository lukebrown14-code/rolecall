# Casey Marlow

Paperbark Software — Backend Engineer

Dear Paperbark Software team,

Reading the ad, the work this hire has to do is finish a migration: moving the last scheduled
batch jobs onto an event-driven design without disturbing payroll exports that venues rely on
every week.

That kind of migration is mostly careful sequencing. At Harbourline Logistics I moved 30
services from hand-built servers to Docker and Terraform on AWS, one service at a time, with
the old path kept running until the new one had proved itself. The same pattern suits batch
jobs that cannot afford a bad run.

The ad also says you deploy many times a day, which only works when the pipeline is fast and
trusted. I rebuilt our CI/CD pipeline in GitHub Actions and cut the deploy time from 45 minutes
to 8 minutes. Earlier, at Fernleaf Digital, I wrote the integration test suite that lifted
coverage from 55% to 85%, which is what made faster releases safe there.

Most of my day-to-day work is building and running Python and Go services. The shipment tracking
API I built in Go and PostgreSQL serves 40 warehouse sites, and I led the on-call rotation that
kept it running. Payroll is new ground for me, and I would want to learn its rules before
touching an export.

Where I would start is the batch jobs themselves: listing what each one reads and writes, so
the move to events can happen in small, reversible steps.

One practical note: I hold full working rights in Australia and can start on four weeks' notice.

Kind regards,
Casey Marlow
