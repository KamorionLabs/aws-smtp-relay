# Deploying AWS SMTP Relay on Amazon EKS

This guide deploys the relay with the [Helm chart](../helm/aws-smtp-relay) and
gives it an IAM role through
[EKS Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html).
No AWS keys are stored in the cluster.

- [Overview](#overview)
- [IAM role and Pod Identity with Terraform](#iam-role-and-pod-identity-with-terraform)
- [IAM role and Pod Identity with the AWS CLI](#iam-role-and-pod-identity-with-the-aws-cli)
- [Install the chart](#install-the-chart)
- [Verify](#verify)
- [Troubleshooting](#troubleshooting)
- [IRSA instead of Pod Identity](#irsa-instead-of-pod-identity)

## Overview

1. The EKS Pod Identity Agent add-on runs on the cluster.
2. An IAM role trusts `pods.eks.amazonaws.com` and may send email with SES.
3. A Pod Identity association links the role to the relay's service account
   (namespace `smtp`, service account `aws-smtp-relay` in this guide).
4. The AWS SDK in the relay gets the role's credentials from the agent.

The association can exist before the chart is installed. If you create it
after, restart the pods so they get the credentials.

The relay calls the SES v2 `SendEmail` API with raw content, for both
`RELAY_API=ses` and `RELAY_API=pinpoint` (Pinpoint Email uses the `ses` IAM
actions). It needs `ses:SendEmail` and `ses:SendRawEmail` on the sending
identities and, if you use one, the configuration set.

## IAM role and Pod Identity with Terraform

```hcl
variable "cluster_name" {
  type = string
}

variable "namespace" {
  type    = string
  default = "smtp"
}

variable "service_account" {
  type    = string
  default = "aws-smtp-relay"
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  ses_arn_prefix = "arn:aws:ses:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}"
}

# Skip this resource if the add-on is already installed on the cluster.
resource "aws_eks_addon" "pod_identity_agent" {
  cluster_name = var.cluster_name
  addon_name   = "eks-pod-identity-agent"
}

data "aws_iam_policy_document" "pod_identity_trust" {
  statement {
    actions = ["sts:AssumeRole", "sts:TagSession"]
    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "smtp_relay" {
  name               = "${var.cluster_name}-aws-smtp-relay"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

data "aws_iam_policy_document" "send_email" {
  statement {
    actions = ["ses:SendEmail", "ses:SendRawEmail"]
    # Narrow down to your verified identities, e.g. "identity/example.com".
    resources = [
      "${local.ses_arn_prefix}:identity/*",
      "${local.ses_arn_prefix}:configuration-set/*",
    ]
  }
}

resource "aws_iam_role_policy" "send_email" {
  name   = "send-email"
  role   = aws_iam_role.smtp_relay.id
  policy = data.aws_iam_policy_document.send_email.json
}

resource "aws_eks_pod_identity_association" "smtp_relay" {
  cluster_name    = var.cluster_name
  namespace       = var.namespace
  service_account = var.service_account
  role_arn        = aws_iam_role.smtp_relay.arn

  depends_on = [aws_eks_addon.pod_identity_agent]
}
```

To deploy the chart with Terraform too, add a `helm_release`:

```hcl
resource "helm_release" "smtp_relay" {
  name             = "aws-smtp-relay"
  chart            = "${path.module}/helm/aws-smtp-relay" # path to the chart
  namespace        = var.namespace
  create_namespace = true

  values = [yamlencode({
    serviceAccount = { name = var.service_account }
    config = {
      awsRegion      = data.aws_region.current.name
      allowToDomains = "example.com"
    }
  })]

  depends_on = [aws_eks_pod_identity_association.smtp_relay]
}
```

### Cross-account sending

To send with identities of another account (`SES_SOURCE_ARN`, `SES_FROM_ARN`,
`SES_RETURN_PATH_ARN`), add their ARNs to `resources` above, and allow the role
in a [sending authorization policy](https://docs.aws.amazon.com/ses/latest/dg/sending-authorization.html)
on each identity in the other account.

## IAM role and Pod Identity with the AWS CLI

```sh
CLUSTER=my-cluster
REGION=eu-west-1
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

# Pod Identity Agent (skip if installed)
aws eks create-addon --cluster-name "$CLUSTER" --addon-name eks-pod-identity-agent

# Role trusted by Pod Identity
aws iam create-role --role-name aws-smtp-relay --assume-role-policy-document '{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {"Service": "pods.eks.amazonaws.com"},
    "Action": ["sts:AssumeRole", "sts:TagSession"]
  }]
}'

# Permission to send email
sed -e "s/REGION/$REGION/g" -e "s/ACCOUNT_ID/$ACCOUNT_ID/g" \
  helm/aws-smtp-relay/examples/iam-policy-ses.json > /tmp/policy.json
aws iam put-role-policy --role-name aws-smtp-relay \
  --policy-name send-email --policy-document file:///tmp/policy.json

# Association with the service account
aws eks create-pod-identity-association \
  --cluster-name "$CLUSTER" \
  --namespace smtp \
  --service-account aws-smtp-relay \
  --role-arn "arn:aws:iam::$ACCOUNT_ID:role/aws-smtp-relay"
```

## Install the chart

The service account name must match the association. With a release named
`aws-smtp-relay`, the default name is `aws-smtp-relay`; set
`serviceAccount.name` otherwise.

```sh
helm install aws-smtp-relay ./helm/aws-smtp-relay \
  --namespace smtp --create-namespace \
  -f helm/aws-smtp-relay/examples/values-eks-pod-identity.yaml
```

The example uses SMTP authentication with a bcrypt hash from an existing
secret:

```sh
kubectl -n smtp create secret generic aws-smtp-relay-auth \
  --from-literal=bcrypt-hash="$(htpasswd -bnBC 10 '' "$PASSWORD" | tr -d ':\n' | sed 's/$2y/$2a/')"
```

Without authentication, restrict who can reach port 1025 with a NetworkPolicy:
any client that reaches the relay can send email with your SES identities.

## Verify

```sh
kubectl -n smtp get pods -l app.kubernetes.io/name=aws-smtp-relay
kubectl -n smtp logs deploy/aws-smtp-relay

# Pod Identity adds these variables to the pod spec
kubectl -n smtp get pod -l app.kubernetes.io/name=aws-smtp-relay \
  -o jsonpath='{.items[0].spec.containers[0].env[*].name}'
# expect AWS_CONTAINER_CREDENTIALS_FULL_URI and AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE
```

Send a test message from a pod in the cluster (the sender must be a verified
SES identity):

```sh
kubectl -n smtp run smtp-test --rm -it --restart=Never --image=python:3-alpine -- \
  python -c '
import smtplib
from email.message import EmailMessage
m = EmailMessage()
m["From"] = "Relay test <sender@example.com>"
m["To"] = "recipient@example.com"
m["Subject"] = "Test from EKS"
m.set_content("Hello from aws-smtp-relay.")
with smtplib.SMTP("aws-smtp-relay.smtp.svc", 1025) as s:
    # s.login("relay", "password")  # with authentication (requires TLS or ENABLE_LOGIN)
    s.send_message(m)
'
```

Each message is logged as JSON on stdout, with an `Error` field on failure.
With metrics enabled, `aws_smtp_relay_messages_total{result="sent"}` counts the
messages sent:

```sh
kubectl -n smtp port-forward svc/aws-smtp-relay 9090 &
curl -s localhost:9090/metrics | grep aws_smtp_relay_
```

## Troubleshooting

| Symptom | Cause and fix |
| ------- | ------------- |
| `NoCredentialProviders`, `failed to refresh cached credentials` | No association for this namespace and service account, or the pod started before it: check `aws eks list-pod-identity-associations --cluster-name $CLUSTER`, then `kubectl -n smtp rollout restart deploy/aws-smtp-relay`. Check that the agent runs: `kubectl -n kube-system get pods -l app.kubernetes.io/name=eks-pod-identity-agent`. |
| `451 4.3.0 ... AccessDeniedException` | The role may not send with this identity or configuration set: fix the IAM policy. The client keeps the message and tries again. |
| `554 5.0.0 ... MessageRejected` | SES refused the message, e.g. an unverified sender or, in the SES sandbox, an unverified recipient. |
| `550 5.7.1` | The sender or a recipient is denied by `allowFrom`, `denyTo`, `allowTo` or `allowToDomains`. |
| Pods not ready | With metrics, the probes use `/healthz` on port 9090; check `kubectl -n smtp describe pod`. |

## IRSA instead of Pod Identity

With [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html),
use a role trusted by the cluster's OIDC provider and annotate the service
account:

```yaml
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/aws-smtp-relay
```
