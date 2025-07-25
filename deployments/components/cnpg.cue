"cloud-native-postgres": {
	alias: "cnpg"
	annotations: {}
	description: "Cloud Native Postgres"
	labels: {
	}
	attributes: workload: {
		definition: {
			apiVersion: "postgresql.cnpg.io/v1"
			kind:       "Cluster"
		}
	}
	type: "component"
}

template: {
	output: {
		apiVersion: "postgresql.cnpg.io/v1"
		kind:       "Cluster"
		metadata: {
			name: parameter.name
		}
		spec: {

			if parameter.description != "" {
				description: parameter.description
			}

			if parameter.imageName == "" {
				imageName: "ghcr.io/cloudnative-pg/postgresql:13.13-13"
			}

			if parameter.instances > 0 {
				instances: parameter.instances
			}
			if parameter.startDelay > 0 {
				startDelay: parameter.startDelay
			}

			if parameter.stopDelay > 0 {
				stopDelay: parameter.stopDelay
			}
			if parameter.primaryUpdateStrategy != "" {
				primaryUpdateStrategy: parameter.primaryUpdateStrategy
			}

			bootstrap: {
				initdb: {
					if parameter.initDatabase != "" {
						database: parameter.initDatabase
					}
					if parameter.initOwner != "" {
						owner: parameter.initOwner
					}

					if parameter.initSecretName != "" {
						secret: {
							name: parameter.initSecretName
						}
					}
				}
			}
			if parameter.enableSuperuser {
				enableSuperuserAccess: parameter.enableSuperuser
				superuserSecret: {
					if parameter.superuserSecretName != "" {
						name: parameter.superuserSecretName
					}
				}
			}
			storage: {
				if parameter.storageClass != "" {
					storageClass: parameter.storageClass
				}
				if parameter.storageSize != "" {
					size: parameter.storageSize
				}
			}

			if parameter.backupPath != "" {
				backup: {
					barmanObjectStore: {
						if parameter.backupPath != "" && parameter.backupEndpointURL != "" {
							destinationPath: parameter.backupPath
							endpointURL:     parameter.backupEndpointURL
							s3Credentials: {
								if parameter.backupAccessKeyID != "" && parameter.backupAccessKeyName != "" {
									accessKeyId: {
										name: parameter.backupAccessKeyName
										key:  parameter.backupAccessKeyID
									}
								}
								if parameter.backupSecretKey != "" && parameter.backupSecretKeyName != "" {
									secretAccessKey: {
										name: parameter.backupSecretKeyName
										key:  parameter.backupSecretKey
									}
								}
							}
							wal: {
								if parameter.walCompression != "" {
									compression: parameter.walCompression
								}
								if parameter.walEncryption != "" {
									encryption: parameter.walEncryption
								}
							}
							data: {
								if parameter.dataCompression != "" {
									compression: parameter.dataCompression
								}
								if parameter.dataEncryption != "" {
									encryption: parameter.dataEncryption
								}
								immediateCheckpoint: parameter.immediateCheckpoint
								if parameter.backupJobs > 0 {
									jobs: parameter.backupJobs
								}
							}
						}
						if parameter.retentionPolicy != "" {
							retentionPolicy: parameter.retentionPolicy
						}
					}
				}
			}

			resources: {
				requests: {
					if parameter.requestMemory != "" {
						memory: parameter.requestMemory
					}
					if parameter.requestCPU != "" {
						cpu: parameter.requestCPU
					}
				}
				limits: {
					if parameter.limitMemory != "" {
						memory: parameter.limitMemory
					}
					if parameter.limitCPU != "" {
						cpu: parameter.limitCPU
					}
				}
			}
			// if parameter.enablePodAntiAffinity && parameter.topologyKey != "" {
			// 	affinity: {
			// 		enablePodAntiAffinity: parameter.enablePodAntiAffinity
			// 		topologyKey:           parameter.topologyKey
			// 	}
			// }
			// nodeMaintenanceWindow: {
			// 	inProgress: parameter.inProgress
			// 	reusePVC:   parameter.reusePVC
			// }
		}
	}

	outputs: {
		if parameter.initSecretName != _|_ {
			secret: {
				apiVersion: "v1"
				kind:       "Secret"
				metadata: {
					name:      parameter.initSecretName
					namespace: context.namespace
				}
				type: "Opaque"
				stringData: {
					username: parameter.initOwner
					password: parameter.initPassword
					database: parameter.initDatabase
				}
			}
		}
	}

	parameter: {

		// +usage=Specify the name of the cluster
		name: string

		// +usage=Provide a description for the cluster
		description: string | *"" // Default to empty string

		// +usage=Specify the image name for PostgreSQL
		imageName: string | *"" // Default to empty string

		// +usage=Set the number of instances
		instances: int | *1 // Default to 0

		// +usage=Specify the start delay in seconds
		startDelay: int | *0 // Default to 0

		// +usage=Specify the stop delay in seconds
		stopDelay: int | *0 // Default to 0

		// +usage=Set the primary update strategy
		primaryUpdateStrategy: string | *"" // Default to empty string

		// +usage=Specify the shared buffers size
		sharedBuffers: string | *"" // Default to empty string

		// +usage=Set the maximum number of statements for pg_stat
		maxStatStatements: string | *"" // Default to empty string

		// +usage=Set the tracking level for pg_stat statements
		trackStatStatements: string | *"" // Default to empty string

		// +usage=Set the minimum log duration for auto explain
		logMinDuration: string | *"" // Default to empty string

		// +usage=Define host-based authentication rules
		pgHba: [{
				type:     string
				database: string
				user:     string
				address:  string
				method:   string
		}] | *[] // Default to empty list

		// +usage=Specify the initial database name
		initDatabase: string | *"" // Default to empty string

		// +usage=Specify the owner of the initial database
		initOwner: string | *"" // Default to empty string

		// +usage=Specify the name of the secret for the initial database
		initSecretName: string | *"" // Default to empty string

		// +usage=Specify the password for the initial database
		initPassword: string | *"" // Default to empty string

		// +usage=Enable or disable superuser access
		enableSuperuser: bool | *false // Default to false

		// +usage=Specify the name of the superuser secret
		superuserSecretName: string | *"" // Default to empty string

		// +usage=Set the storage class
		storageClass: string | *"" // Default to empty string

		// +usage=Define the storage size
		storageSize: string | *"" // Default to empty string

		// +usage=Specify the backup destination path
		backupPath: string | *"" // Default to empty string

		// +usage=Set the backup endpoint URL
		backupEndpointURL: string | *"" // Default to empty string

		// +usage=Specify the access key ID for backup
		backupAccessKeyID: string | *"" // Default to empty string

		// +usage=Specify the name of the secret containing the access key ID for backup
		backupAccessKeyName: string | *"" // Default to empty string

		// +usage=Specify the secret access key for backup
		backupSecretKey: string | *"" // Default to empty string

		// +usage=Specify the name of the secret containing the secret access key for backup
		backupSecretKeyName: string | *"" // Default to empty string

		// +usage=Set the compression method for WAL
		walCompression: string | *"" // Default to empty string

		// +usage=Set the encryption method for WAL
		walEncryption: string | *"" // Default to empty string

		// +usage=Set the compression method for data
		dataCompression: string | *"" // Default to empty string

		// +usage=Set the encryption method for data
		dataEncryption: string | *"" // Default to empty string

		// +usage=Specify whether to perform an immediate checkpoint
		immediateCheckpoint: bool | *false // Default to false

		// +usage=Set the number of backup jobs
		backupJobs: int | *0 // Default to 0

		// +usage=Specify the retention policy for backup
		retentionPolicy: string | *"" // Default to empty string

		// +usage=Set the requested memory
		requestMemory: string | *"" // Default to empty string

		// +usage=Set the requested CPU
		requestCPU: string | *"" // Default to empty string

		// +usage=Set the memory limit
		limitMemory: string | *"" // Default to empty string

		// +usage=Set the CPU limit
		limitCPU: string | *"" // Default to empty string

		// +usage=Enable or disable pod anti-affinity
		enablePodAntiAffinity: bool | *false // Default to false

		// +usage=Specify the topology key for pod anti-affinity
		topologyKey: string | *"" // Default to empty string

		// +usage=Set the node maintenance window in progress state
		inProgress: bool | *false // Default to false

		// +usage=Specify whether to reuse PVC during maintenance
		reusePVC: bool | *false // Default to false
	}
}
