/**
 * @fileoverview
 * @enhanceable
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!

goog.provide('proto.sql.ExecContextRsp');

goog.require('jspb.BinaryReader');
goog.require('jspb.BinaryWriter');
goog.require('jspb.Message');


/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.sql.ExecContextRsp = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.sql.ExecContextRsp, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  proto.sql.ExecContextRsp.displayName = 'proto.sql.ExecContextRsp';
}


if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto suitable for use in Soy templates.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     com.google.apps.jspb.JsClassTemplate.JS_RESERVED_WORDS.
 * @param {boolean=} opt_includeInstance Whether to include the JSPB instance
 *     for transitional soy proto support: http://goto/soy-param-migration
 * @return {!Object}
 */
proto.sql.ExecContextRsp.prototype.toObject = function(opt_includeInstance) {
  return proto.sql.ExecContextRsp.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Whether to include the JSPB
 *     instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.sql.ExecContextRsp} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.sql.ExecContextRsp.toObject = function(includeInstance, msg) {
  var f, obj = {
    affectedrows: jspb.Message.getFieldWithDefault(msg, 1, 0),
    lastid: jspb.Message.getFieldWithDefault(msg, 2, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.sql.ExecContextRsp}
 */
proto.sql.ExecContextRsp.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.sql.ExecContextRsp;
  return proto.sql.ExecContextRsp.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.sql.ExecContextRsp} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.sql.ExecContextRsp}
 */
proto.sql.ExecContextRsp.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setAffectedrows(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setLastid(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.sql.ExecContextRsp.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.sql.ExecContextRsp.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.sql.ExecContextRsp} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.sql.ExecContextRsp.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAffectedrows();
  if (f !== 0) {
    writer.writeInt64(
      1,
      f
    );
  }
  f = message.getLastid();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
};


/**
 * optional int64 affectedRows = 1;
 * @return {number}
 */
proto.sql.ExecContextRsp.prototype.getAffectedrows = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/** @param {number} value */
proto.sql.ExecContextRsp.prototype.setAffectedrows = function(value) {
  jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional int64 lastId = 2;
 * @return {number}
 */
proto.sql.ExecContextRsp.prototype.getLastid = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/** @param {number} value */
proto.sql.ExecContextRsp.prototype.setLastid = function(value) {
  jspb.Message.setProto3IntField(this, 2, value);
};

