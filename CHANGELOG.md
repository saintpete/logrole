# Changes

## 1.1

Tweak the README/homepage and describe how to contribute to the project.

## 1.0

We made it! Link from every phone number on the site to the phone number
instance view.

## 0.77

Flush out the phone number instance view - show calls and messages to and from
this number. Add links to the fuller list of those resources. Show dates in
a shorter format if a message is old.

## 0.76

Flush out the phone number list view (paging, filters). Implement a phone
number instance view. Redirect /phone-numbers/PN123 to /phone-numbers/+1410...
and implement tab-to-search.

## 0.73

Add a phone number list view.

## 0.72

Reject invalid query parameters on list views

Implement per-user/group MaxResourceAge settings.

Refactor template generation a little bit.

## 0.71

Implement caching for Messages

Show whether a result was returned from the cache, and if so how old that
result is. Alters services.Duration to show fewer bits after the decimal if the
Duration is larger than one second.

## 0.70

Implement search filters for Alerts

## 0.69

Add back caching for calls/messages/conferences

## 0.67

Switch the date filtering system from day-and-UTC based to timezone aware, to
the hour filtering of calls, messages, and conferences.

## 0.66

Cosmetic changes - changing "redacted" to "hidden" and reordering some filters.

## 0.61

Implement an Alerts list view.

## 0.58

Implement multi-user permissions

Specify `policy` or `policy_file` to define permission groups, and define users
to exist in those groups, as well as a default group for unknown users. Support
policy in the GoogleAuthenticator and the BasicAuthAuthenticator. Document how
policy behaves if it is/isn't specified, and how it interacts with
allowed_domains.

Moves all of the Authenticator code into the config directory. The interaction
between GoogleAuthenticator and the server directory is a little complicated -
we want to render a 401 error if Google auth fails, which needs to be done from
the server directory, but GoogleAuthenticator shouldn't necessarily live there.
We also need to get a URL from GoogleAuthenticator to show on the login page.
We hack around this, I'm not super happy with it, but it works for the moment.
Open to better ideas about how this should work.

Move the YAML config out of the logrole_server binary and into the config
directory. Add Policy to it, and a custom parser for UserSettings. These let
other Go code load a Logrole YAML file, if they want.

Add more documentation around possibly-confusing settings. Document how to get
a Google client ID and client secret.

Fix errors in Google authentication, and add a whole bunch of tests around
policies, Google auth, and Basic auth. Removes some unused code that set
a global map in the config directory.

Add tools in write_config_from_env to download a policy file from a URL (for
Heroku deployment, if you can't include the permissions as part of the Git
repo).

## 0.56

Highlight Call list rows in red if the call ended unsuccessfully.

## 0.55

Show error/warning information about a Call on the instance page.

Messages that resulted in an error are highlighted in red.

## 0.54

You can configure timezones via config.yml, and the timezones in the menu bar
are now dynamic.

## 0.51

Gzip static files so they get sent to the client more quickly. It would be nice
to also gzip the HTML, but this would be vulnerable to BEAST/CRIME attacks on
SSL.

## 0.50

Add a Conference instance view.

## 0.48

Renamed the binaries from `server` and `write_config_from_env` to
`logrole_server` and `logrole_write_config_from_env` to avoid conflicts with
other Go binaries.

Add `google_allowed_domains` config variable to restrict access to email
addresses that are part of a certain domain.
