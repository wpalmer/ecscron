### ECSCron

crontab-like functionality using AWS ECS "run-task"

When using ECS, it is often the case that all you want out of cron is to
kick-off ECS tasks. Logging, monitoring, notifications, etc, are usually
handled by other systems. Running "real cron" for this purpose is a mix
of overkill and not enough: as the bulk of all tasks will be run
elsewhere, cron won't receive any useful logging, and meanwhile it's
hard to get status messages out of cron itself when debugging.
Meanwhile, the memory footprint of a full cron system is hard to
predict: because cron runs all tasks in parallel, memory requirements
can easily baloon briefly, once per minute, followed by a majority of
time spent idle. This makes memory reservation extremely inefficient.

ECSCron attempts to alleviate all of these issues:

 - ECSCron has only one job: calling ECS Run-Task.
 - It always runs in the foreground, easing containerization.
 - It can output noisy logs to STDOUT, for ease of debugging.
 - It doesn't run anything in parallel, so the memory footprint is
   consistent.

#### crontab format

The crontab format is meant to be roughly the same as UNIX crontab files.
Environment variables are not supported, nor is any equivalent to the
"user" column seen in system-wide crontabs. Only a single-word `command`
column is allowed on each line: the name of the ECS Task to run.

Leading/trailing whitespace, as well as anything after a `#`, is ignored.

Example, running the "HelloWorld" task once every five minutes, between
the hours of 9am and 6pm, Monday through Friday:
    # minute  hour    day-of-month  month   day-of-week task
      */5     9-17    *             *       1-5         HelloWorld

see `man 5 crontab` for more information on the time specfication format.

#### Running

Basic Usage:

    ecscron -crontab my-crontab-file -cluster my-cluster -region eu-west-1

Via Docker:

    docker run \
      -v "$HOME/.aws:/root/.aws" \
      -e "HOME=root" \
      -v "$PWD/my-crontab-file:/etc/ecscrontab"
      wpalmer/ecscron \
      -cluster my-cluster \
      -region eu-west-1

Arguments:

 * `-help` A usage message (which may be more up-to-date than this document)
 * `-async <YYYY-MM-DD HH:mm:ss>`
   The "last run" of cron (to resume after interruption) in
   `YYYY-MM-DD HH:mm:ss` format. Any tasks which would have run between
   the specified time and "now", will run immediately (duplicates are
   supressed). Time is evaluated in the timezone given by the
   `-timezone` option.
 * `-cluster <ECS Cluster ID>`
   The ECS Cluster on which to run tasks.
 * `-crontab <filename>`
   The location of the crontab file to parse (default "/etc/ecscrontab").
 * `-debug <level number>` Debug level
   * 0 = errors/warnings
   * 1 = run info
   * 2 = detail
   * 5 = status
 * `-prefix <string>`
   An optional prefix to add to all ECS Task names within the crontab.
   This may be useful for switching between environments or versions
   without editing the crontab.
 * `-region <AWS Region Identifier>`
   The AWS Region in which the ECS Cluster resides.
 * `-simulate <true|false>`
   When true, don't actually run anything, only print what would be run.
 * `-suffix <string>`
   An optional suffix to add to all ECS Task names within the crontab.
   This may be useful for switching between environments or versions
   without editing the crontab.
 * `-timezone <identifier>`
   The TimeZone in which to evaluate cron expressions (default "UTC").
