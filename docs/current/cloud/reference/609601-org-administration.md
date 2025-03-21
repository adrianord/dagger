---
slug: /cloud/609601/org-administration
---

# Organization Administration

## Members

A member account grants a person access to log in to a Dagger Cloud organization to diagnose pipeline failures and collaborate on changes. Member information shown on the *All Runs* and *All Changes* pages is populated by the VCS you have integrated with Dagger Cloud. Deleting a member of a Dagger Cloud organization will not remove their runs and changes from Dagger Cloud.

:::note
You must hold the *Admin* role of a Dagger Cloud organization to administer members. You cannot change a member's role. Please contact Dagger Support via the in-app messenger for assistance if you need to change a member's role. This functionality is coming soon.
:::

You can:

* Add a member to the organization
* Delete a member from the organization

### Add a member to an organization

1. Browse to the *Organizations Settings* page of the Dagger Cloud dashboard (accessible by clicking your user profile icon in the Dagger Cloud interface). Select your organization and navigate to the *Members* tab.
1. Click *Add member* and then enter the email address for the member you would like to add.
1. Click *Add another* to invite additional members.
1. Click *Send Invites* when you are done.

![Send invitations](/img/current/cloud/reference/org-administration/invite-members.png)

Each person will then receive an email invitation. Once they accept the invitation, they will be added to the organization. New users are added to the *Member* role by default.

### Delete a member from an organization

1. Browse to the *Organizations Settings* page of the Dagger Cloud dashboard (accessible by clicking your user profile icon in the Dagger Cloud interface). Select your organization and navigate to the *Members* tab.
1. In the list of members, click the *Delete* icon for the member.
