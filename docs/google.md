## Google OAuth

Google OAuth can be slightly tricky to configure. Fortunately we take care of
most of the details for you.

## 1. Create a Project

Go to this URL: https://console.developers.google.com/projectselector/apis/credentials

<img src="https://kev.inburke.com/blog/images/logrole-google-credentials.png" />

Create a new project. Name it whatever you want.

## 2. Create Credentials

Click the "Create Credentials" dropdown. Select "OAuth Client ID" - "for API's
like Google Calendar".

<img src="https://kev.inburke.com/blog/images/logrole-create-creds-screen.png" />

On the next screen select 'Web Application'. Name it whatever you want.

Omit the "Authorized Javascript Origins" - you don't need this, there's no
Javascript. For "Authorized redirect URI's", you need to specify whatever URL's
users will see in their browser, **plus** the path `/auth/callback`. I use
these:

- http://localhost:4114/auth/callback               (for development)
- https://logrole.herokuapp.com/auth/callback       (my production site)

You *must* include the `/auth/callback` part, or the Google Authenticator in
Logrole won't work properly.

<img src="https://kev.inburke.com/blog/images/logrole-create-callback-url.png" />

On the next screen, you should be presented with a client ID and a
client secret. Put these in your `config.yml` as `google_client_id` and
`google_client_secret`, respectively. The ID should be a lot longer than the
secret.
