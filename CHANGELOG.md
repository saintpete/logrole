# Changes

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
