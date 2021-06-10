terraform {
  required_providers {
    sealedsecrets = {
      source = "tunein.com/tunein-incubator/sealedsecrets"
      version = "0.0.1"
    }
  }
}

provider aws {
  region = "us-west-2"
  version = "~> 3.21"
}

data aws_eks_cluster_auth current {
  name = data.aws_eks_cluster.current.id
}

data aws_eks_cluster current {
  name = "operations-nonprod-uswest2-02"
}

provider sealedsecrets {
  server_address = data.aws_eks_cluster.current.endpoint
}

data sealedsecrets_sealed_secret current {
  name = "test"
  data = {
    foo = "bar"
  }
  scope = "cluster-wide"
}

resource "null_resource" current {
  depends_on = [data.sealedsecrets_sealed_secret.current]
}

output "manifest" {
  value = data.sealedsecrets_sealed_secret.current.manifest
}
