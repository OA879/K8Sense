[API](../API.md) / [plugin/registry](../modules/plugin_registry.md) / K8senseEvent

# Interface: K8senseEvent<EventType\>

[plugin/registry](../modules/plugin_registry.md).K8senseEvent

Represents a K8sense event. It can be one of the default events or a custom event.

## Type parameters

| Name | Type |
| :------ | :------ |
| `EventType` | `K8senseEventType` \| `string` |

## Hierarchy

- **`K8senseEvent`**

  ↳ [`DeleteResourceEvent`](plugin_registry.DeleteResourceEvent.md)

## Properties

### data

• `Optional` **data**: `unknown`

#### Defined in

[redux/k8senseEventSlice.ts:69](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L69)

___

### type

• **type**: `EventType`

#### Defined in

[redux/k8senseEventSlice.ts:68](https://github.com/kubernetes-sigs/k8sense/blob/072d2509b/frontend/src/redux/k8senseEventSlice.ts#L68)
