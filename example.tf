variable data {
  type = map(string)
  default = {
    foo = "bar"
  }
}

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

data aws_eks_cluster current {
  name = "operations-nonprod-uswest2-02"
}

provider sealedsecrets {
  server_address = data.aws_eks_cluster.current.endpoint
}

resource sealedsecrets_sealed_secret current {
  name = "test"
  data = var.data
  scope = "cluster-wide"
}

output "manifest" {
  value = sealedsecrets_sealed_secret.current.manifest
}

output "data" {
  value = var.data
}
