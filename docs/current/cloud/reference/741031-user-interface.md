---
slug: /cloud/741031/user-interface
---

# User Interface

Dagger Cloud lets you visualize your Dagger pipelines in two ways: runs and changes.

## Runs

A run is an invocation of a Dagger pipeline. If you have many CI jobs configured to run for every pull/merge request or commit, you may see several runs for a single commit or single pull/merge request.

The *All Runs* page lists available runs, as shown below:

![View runs](/img/current/cloud/reference/user-interface/all-runs.png)

Here's a quick summary of what you'll see for each run.

|Field | Description |
|---|---|
|Status | Indication of run success or failure |
|Title | Change or commit title (abbreviated) |
|Commit | Commit ID |
|Change | Reference to change in source code repository |
|Start | Run start time |
|Duration | Run duration |
|User | User triggering the run |
|Runner Job | Run ID (first 8 identifying bytes) |
|Branch | Name of the branch in source code repository |
|Remote | Full path to the remote source code repository |

:::tip
You can display a subset of runs, such as runs related to a specific commit, branch, user or remote, by clicking the *Filter* icon in the corresponding field of the run list.
:::

You can drill down into the details of a specific run by clicking it. This directs you to a run-specific *Run Details* page, as shown below:

![View run details](/img/current/cloud/reference/user-interface/run-details-tree.png)

The *Run Details* page includes detailed status and duration metadata about the pipeline steps. The tree view shows Dagger pipelines and steps within those pipelines. If there are any errors in the run, Dagger Cloud automatically brings you to the first error in the list. You see detailed logs and output of each step, making it easy for you to debug your pipelines and collaborate with your teammates.

:::note
It is also possible to view a run as a Directed Acyclic Graph (DAG). This DAG view displays a graph of everything that happened in a Dagger run and shows the status for each step. This DAG view is currently experimental; to activate it, visit the *User Settings* page of the Dagger Cloud dashboard (accessible by clicking your user profile icon in the Dagger Cloud interface) and turn on the *Use DAG view features for viewing runs* option. You will now have an option to toggle between the standard tree view and the experimental DAG view on the *Run Details* page. Here is an example of the DAG view for a run:

![View run details](/img/current/cloud/reference/user-interface/run-details-dag.png)
:::

## Changes

A change is a group of runs for a specific commit or pull/merge request.

The *All Changes* page lists available groups, as shown below:

![View changes](/img/current/cloud/reference/user-interface/all-changes.png)

Here's a quick summary of what you'll see for each change.

|Field | Description |
|---|---|
|Status | Indication of run success or failure |
|Title | Change or commit title (abbreviated) |
|Commit | Commit ID |
|Change | Reference to change in source code repository |
|Start | Run start time |
|Duration | Run duration |
|User | User triggering the run |
|Runner Job | Run ID (first 8 identifying bytes) |
|Branch | Name of the branch in source code repository |
|Remote | Full path to the remote source code repository |

You can drill down into the details of a specific change by clicking it. This directs you to a *Change Details* page, as shown below:

![View change details](/img/current/cloud/reference/user-interface/change-details.png)

The *Change Details* page lists all the pipeline runs for the commit or pull request. The tree view shows Dagger pipelines and detailed logs of steps and outputs within those pipelines.
