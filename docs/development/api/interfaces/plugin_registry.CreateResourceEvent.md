[API](../API.md) / [plugin/registry](../modules/plugin_registry.md) / CreateResourceEvent

# Interface: CreateResourceEvent

[plugin/registry](../modules/plugin_registry.md).CreateResourceEvent

Event fired when creating a resource.

## Properties

### data

• **data**: `Object`

#### Type declaration

| Name | Type | Description |
| :------ | :------ | :------ |
| `status` | `CONFIRMED` | What exactly this event represents. 'CONFIRMED' when the user chooses to apply the new resource. For now only 'CONFIRMED' is sent. |

#### Defined in

[redux/k8senseEventSlice.ts:193](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L193)

___

### type

• **type**: `CREATE_RESOURCE`

#### Defined in

[redux/k8senseEventSlice.ts:192](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L192)
