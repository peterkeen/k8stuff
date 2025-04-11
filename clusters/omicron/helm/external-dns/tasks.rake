task :create_dynamodb_table do
  validate_tool("aws")

  cmd = %w[
    aws
    dynamodb 
    create-table
    --table-name external-dns
    --attribute-definitions AttributeName=k,AttributeType=S
    --key-schema AttributeName=k,KeyType=HASH
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5
    --table-class STANDARD
  ]

  sh cmd.join(" ")
end

task :scan_dynamodb_table do
  sh "aws dynamodb scan --table-name external-dns"
end
