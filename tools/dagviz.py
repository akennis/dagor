import json
import sys
import os
from graphviz import Digraph
import argparse


def load_dag_structure(file_path):
    """Load and parse DAG structure from JSON file."""
    if not file_path.endswith(".json"):
        print("Unsupported dag file format")
        sys.exit(1)
    
    with open(file_path, 'r') as f:
        dag_content = json.load(f)
    return dag_content


def get_node_color(node_type, type_palette):
    """Determine the color for a node based on its type."""
    if not node_type:
        return ""
    node_type_lower = node_type.lower()
    if node_type_lower not in type_palette:
        raise ValueError("Unknown node type: {}".format(node_type))
    return type_palette[node_type_lower]


def create_graph_nodes(diagram, node_collection, type_palette):
    """Create all nodes in the graph and return registry mappings."""
    node_registry = {}
    output_field_registry = {}
    
    for vertex_id, vertex_config in node_collection.items():
        node_color = ""
        if "type" in vertex_config:
            node_color = get_node_color(vertex_config["type"], type_palette)
        
        incoming_fields = vertex_config.get("inputs", {})
        is_source_node = incoming_fields is None or len(incoming_fields) == 0
        
        if is_source_node:
            diagram.node(vertex_id, vertex_id, style="filled", shape="oval", fillcolor=node_color)
        else:
            diagram.node(vertex_id, vertex_id, fillcolor=node_color)
        
        outgoing_fields = vertex_config.get("outputs", {})
        if outgoing_fields is None:
            outgoing_fields = {}
        
        for output_var, output_field in outgoing_fields.items():
            output_field_registry[output_field] = vertex_id
        
        node_registry[vertex_id] = vertex_config
    
    return node_registry, output_field_registry


def create_graph_edges(diagram, node_registry, output_field_registry):
    """Create all edges in the graph based on node dependencies."""
    for vertex_id, vertex_config in node_registry.items():
        incoming_fields = vertex_config.get("inputs", {})
        
        for input_var, input_field in incoming_fields.items():
            if vertex_id not in node_registry:
                raise AssertionError("dependency node not found: {}".format(vertex_id))
            if input_field not in output_field_registry:
                raise AssertionError("dependency output not found: {}".format(input_field))
            
            source_vertex = output_field_registry[input_field]
            diagram.edge(source_vertex, vertex_id, input_field)


def generate_visualization(dag_content, graph_direction, output_path):
    """Generate the complete graph visualization."""
    if "vertices" not in dag_content:
        raise AssertionError("vertices not found")
    
    node_collection = dag_content["vertices"]
    
    type_palette = {
        "io": "#b8f3a9",
        "calc": "#9fd7f5",
        "mix": "#cfa3f0",
        "filter": "#ffe1be",
    }
    
    diagram = Digraph()
    diagram.attr("node", shape="box", style="filled, rounded")
    diagram.attr(rankdir=graph_direction)
    
    node_registry, output_field_registry = create_graph_nodes(diagram, node_collection, type_palette)
    create_graph_edges(diagram, node_registry, output_field_registry)
    
    base_filename, file_extension = os.path.splitext(output_path)
    diagram.render(filename=base_filename, view=False, format=file_extension[1:], cleanup=True)


if __name__ == "__main__":
    arg_parser = argparse.ArgumentParser(description="DAG visualization tool")
    arg_parser.add_argument("-i", "--input_file", type=str, required=True, help="dag json file path")
    arg_parser.add_argument("-o", "--output_file", type=str, required=True, help="output image file")
    arg_parser.add_argument("-d", "--direction", type=str, default="TB", choices=["LR", "RL", "TB", "BT"], required=False, help="direction of the graph, default is TB")
    parsed_args = arg_parser.parse_args()
    
    dag_data = load_dag_structure(parsed_args.input_file)
    generate_visualization(dag_data,parsed_args.direction, parsed_args.output_file)
