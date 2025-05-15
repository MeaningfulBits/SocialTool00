Uses go-mastodon and the mastodon api to get and output three lists (Followers, Following, Mutuals) from the linked mastodon account.

Usage:
You'll need to replace some of the info in the conf file.

Replace "clientKey", "clientSecret", "accessToken", and "serverURL"

You can find more information by visiting the 'settings/applications' section for your mastodon account.

After updating client info you can run the program using the syntax below or build the program and use it with scripts, cron, or other programs/applications.

go run main.go ./conf/STDefault.conf

