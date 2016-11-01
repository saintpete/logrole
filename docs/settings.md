# Logrole Settings

There are two main ways to deploy Logrole.

1. Write all settings to a `config.yml` file (a sample is in
config.sample.yml), then run `logrole_server --config=config.yml`.

2. Set all configuration as environment variables. Run
`logrole_write_config_from_env > config.yml` to write all those environment
variables to a config.yml file. Follow the steps in (1).

### Environment variables

Some environments like Heroku only allow you to set production
configuration via environment variables. Logrole has a second binary,
`logrole_write_config_from_env`, that takes named environment variables and
writes them to a YAML file. You can then run the `logrole_server` binary as
described above.

```
logrole_write_config_from_env > myconfig.yml
logrole_server --config=myconfig.yml
```

To see which environment variables are written to the file, run
`logrole_write_config_from_env --help`. Here is an example:

```
Supported environment variables are:

PORT                   Port to listen on
PUBLIC_HOST            Host your users will browse to to see the site

TWILIO_ACCOUNT_SID     Account SID for your Twilio account
TWILIO_AUTH_TOKEN      Auth token

REALM                  Realm (either "local" or "prod")
TZ                     Default timezone (example "America/Los_Angeles")
TIMEZONES              Comma-separated list of timezones users can choose from
                       (example "America/New_York,UTC,America/Chicago").
                       Defaults to the four US timezones.
EMAIL_ADDRESS          For "Contact Support" on server error pages
PAGE_SIZE              How many resources to fetch/display on each page

SECRET_KEY             64 byte hex key - generate with "openssl rand -hex 32"
MAX_RESOURCE_AGE       How long resources should be visible for - "720h" to
                       hide anything older than 30 days
SHOW_MEDIA_BY_DEFAULT  "false" to hide images behind a toggle when a user
                       browses to a MMS message.

AUTH_SCHEME            "basic", "noop", or "google"
BASIC_AUTH_USER        For basic auth, the username
BASIC_AUTH_PASSWORD    For basic auth, the password
GOOGLE_CLIENT_ID       For Google OAuth
GOOGLE_CLIENT_SECRET   For Google OAuth
GOOGLE_ALLOWED_DOMAINS Comma separated list of domains to allow to
                       authenticate. If empty or omitted, all domains allowed.

ERROR_REPORTER         "sentry", empty, or register your own.
ERROR_REPORTER_TOKEN   Token for the error reporter.

POLICY_FILE            Load policy info from a file
POLICY_URL             Download policy info from the specified URL. HTTPS only.
                       Can be protected with Basic Auth. Consider using Dropbox
                       or Github Gist "raw" URLs.
```

#### Heroku deployment

Logrole comes with a default Procfile and a start script (`bin/serve`) for easy
Heroku deployment. Sensitive environment variables (auth token, basic auth
password, etc) are dropped before the server process starts.

## Settings details

### Secret key

The secret key is used to obscure URL's with sensitive information (such as
NextPageURI's and MMS URL's), to encrypt cookies, and to ensure valid OAuth
sessions. It should be 32 bytes of cryptographically random data. Because some
bytes are unprintable garbage, it's easier to store this value in a file as a
64-byte hex-encoded value.

OpenSSL can generate random bytes for you. Type `openssl rand -hex 32` and you
will get a value like this:

```
73cfe0f6926d3b3600b420dontuse20dbe775c1a8e221c72070e5362516c0a34
```

That's the format of a secret key that you should pass as a `secret_key` to
Logrole.

### Timezones

When was a call or SMS made? You can choose both a default timezone for your
server, and then also a list of timezones that users can select from. All times
will be displayed in that local zone.

Timezones are specified in the format used by [the IANA timezone
database][iana], for example "America/Los_Angeles". [A partial list may be
found at Wikipedia][tz-list]. Note, your server may not have every timezone
listed there.

In a YML file, specify `default_timezone: America/Los_Angeles` to configure
a default timezone, and a list of `timezones` to configure the available
timezones in the menu bar.

```yml
timezones:
  - Asia/Singapore
  - Asia/Tokyo
  - Europe/London
  - America/New_York
  - Africa/Cairo
```

[iana]: https://en.wikipedia.org/wiki/Tz_database
[tz-list]: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones

## Max Resource Age

You may want to prohibit viewers from seeing a resource older than a certain
age. Specify a `max_resource_age` in your config file to limit the ability to
view resources.

We parse the max resource age using [Go's `time.Duration`
type][parse-duration]. Note that this type does not support days or months,
since a day may have 23 or 25 hours. To specify "30 days", specify the max
resource age as "720h", or `30*24` hours.

```
max_resource_age: 720h
```

[parse-duration]: https://golang.org/pkg/time/#ParseDuration

## Authentication

Logrole supports three different methods of authentication, via the
`auth_scheme` parameter in your YAML file.

### No Authentication

Set `auth_scheme: noop` to let everyone visit your site. By default all
visitors will be able to see everything. (If you are manually configuring
Logrole, you can set the `User` field on the `NoopAuthenticator` to a different
permission set.)

### Basic Authentication

Set `auth_scheme: basic` to enforce access to your site with HTTP Basic Auth.
You can set a basic auth user and password in your config file like so:

```yml
basic_auth_user: test
basic_auth_password: hymanrickover
```

You can specify permissions for the Basic Auth user by adding a policy file,
described below. If that file is not present, permissions for the DefaultUser
are given to all users.

### Google Authentication

Set `auth_scheme: google` to use Google OAuth Authentication. Users will be
redirected to Google to login, and then sent back to Logrole.

You'll need a Google Client ID and Client Secret for OAuth. [Follow these
instructions][google] to get those values.

[google]: https://github.com/saintpete/logrole/tree/master/docs/google.md

#### Allowed domains

You can configure an array of domains that are allowed to access the site. For
example, if you specify "example.com", only emails that end with @example.com
will be allowed to access the site.

```yml
google_allowed_domains:
  - example.com
  - example.net
  - example.org
```

## Custom permissions for different groups

Use a `policy` to define groups with different permissions. Your `policy` will
look something like this, in YAML:

```yml
policy:
    - name: support
      default: true
      permissions:
          can_view_message_body: false
          can_play_recordings: false
          can_view_message_price: false
          can_view_call_price: false
      users:
          - test@example.com
          - test@example.net

    - name: eng
      permissions:
          can_view_call_price: False
      users:
          - eng@example.com
          - eng@example.net
```

Let's walk through that:

- **name:** the group name. Required

- **default:** If a user's email is in the group of allowed_domains, but they're
not explicitly specified in a group in the policy, they'll get the permissions
of a `default` group. Only one group can be the default.

- **permissions:** A list of permissions that this group has. Permissions are
**set to true by default,** so you only need to specify the permissions you
want to disallow. A full list of permissions and descrptions can be found on
[the UserSettings object][user-settings].

- **users:** A list of users in this group. These should match the id provided
  for Basic Auth, or the email address used to sign in with Google. A user
  cannot belong to two different groups.

#### Edge cases

There are two tools for locking down access to your site - configuring the
email domains that are allowed to access the site, and specifying a policy. You
don't have to specify those, and they can interact in surprising ways. We try
our best to do the intuitive thing. Here's a short walkthrough of how Logrole
handles different cases.

If a the user's email address is found in the policy, that user is used.

If no policy is defined, we use google_allowed_domains to determine access,
and return config.DefaultUser for user access for all authenticated users.

If no policy is defined and google_allowed_domains is empty, we permit all
users to access the site.

If google_allowed_domains is not empty and a user's domain is not allowed to
access the site, they are denied access.

If google_allowed_domains is not empty, and a user's domain is allowed, but
they are not in a group, we use the permissions for the default group. If no
default group exists, the user is denied access.

[user-settings]: https://godoc.org/github.com/saintpete/logrole/config#UserSettings

### What happens to the YAML file?

You don't need to read this if you are just running logrole_server. But if you
want to configure Logrole from another Go app, these details are helpful.

In the `logrole_server` binary three things happen:

1. Parse the YAML file into a config.FileConfig struct in memory.

- From there, call config.NewSettingsFromConfig, which initializes things like
the Twilio Client, and an Authenticator, and gives you back a Settings object.

- From there, call `s, _ := server.NewServer(settings)` and you'll get back
  a HTTP handler. You can then use this to listen on a socket and serve HTTP.

```go
settings := &server.Settings{PublicHost: "example.com", PageSize: 50}
s, err := server.NewServer(settings)
http.Handle("/", s)
http.ListenAndServe(":4114", nil)
```

The logrole_server binary handles these automatically for you. If you are
writing custom code, you can skip any or all of these steps with your own
initialization.

[settings-godoc]: https://godoc.org/github.com/saintpete/logrole/server/#Settings
