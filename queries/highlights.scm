(script_declaration name: (identifier) @type)
(component_declaration name: (identifier) @type)
(system_declaration name: (identifier) @function)
(signal_declaration name: (identifier) @function)

(import_statement path: (import_path (identifier) @module))
(import_path (identifier) @module)

(inspect_field name: (identifier) @property)
(node_field name: (identifier) @property)
(nodes_field name: (identifier) @property)
(resource_field name: (identifier) @property)
(bare_field name: (identifier) @property)
(reactive_field name: (identifier) @property)
(derived_field name: (identifier) @property)
(component_field name: (identifier) @property)

(handler_parameter name: (identifier) @variable.parameter)
(query_parameter name: (identifier) @variable.parameter)
(watch_target field: (identifier) @property)

(signal_path script: (identifier) @type)
(signal_path name: (identifier) @function)

(type_expression (identifier) @type)
(param_type_expression (identifier) @type)
