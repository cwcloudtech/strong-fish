import 'package:dio/dio.dart';
import 'package:http_parser/http_parser.dart';

import 'api_client.dart';
import 'api_exception.dart';
import '../models/models.dart';

/// The API surface, grouped by domain. Mirrors sf-ui's api/services.js so both
/// clients call the same endpoints in the same shapes.
class SfApi {
  final ApiClient client;

  SfApi(this.client);

  Map<String, dynamic> _map(dynamic data) => data is Map<String, dynamic> ? data : const {};

  List<Map<String, dynamic>> _list(dynamic data) =>
      (data as List? ?? const []).whereType<Map<String, dynamic>>().toList();

  // --- config & auth ---

  Future<Map<String, dynamic>> config() async => _map((await client.dio.get('/config')).data);

  Future<LoginResult> login(String email, String password) async {
    final response = await client.dio.post('/users/login', data: {'email': email, 'password': password});
    return LoginResult.fromJson(_map(response.data));
  }

  /// Opens an account. [coach] records a claim, not a grant: a superadmin
  /// confirms it, and the account is an athlete until they do.
  Future<LoginResult> register({
    required String email,
    required String password,
    required String name,
    required String surname,
    required String locale,
    bool coach = false,
  }) async {
    final response = await client.dio.post('/users', data: {
      'email': email,
      'password': password,
      'name': name,
      'surname': surname,
      'coach': coach,
      'locale': locale,
    });
    return LoginResult.fromJson(_map(response.data));
  }

  Future<User> me() async => User.fromJson(_map((await client.dio.get('/users/me')).data));

  Future<User> updateProfile(Map<String, dynamic> payload) async =>
      User.fromJson(_map((await client.dio.put('/users/me', data: payload)).data));

  Future<User> updatePicture(String picture) async => User.fromJson(
      _map((await client.dio.put('/users/me/picture', data: {'picture': picture, 'x': 50, 'y': 50})).data));

  Future<void> forgotPassword(String email) =>
      client.dio.post('/users/forgot-password', data: {'email': email});

  /// Finishes an MFA-gated login with an authenticator-app code. Security keys
  /// are web-only: the WebAuthn ceremony needs a browser, so the app offers TOTP
  /// and tells the user to use the web app for their key.
  Future<LoginResult> loginTotp(String challengeToken, String code) async {
    final response = await client.dio.post('/users/login/mfa/totp', data: {
      'challengeToken': challengeToken,
      'code': code,
    });
    return LoginResult.fromJson(_map(response.data));
  }

  // --- exercises & 1RMs ---

  Future<List<Exercise>> exercises() async =>
      _list((await client.dio.get('/exercises')).data).map(Exercise.fromJson).toList();

  Future<List<OneRm>> oneRms() async =>
      _list((await client.dio.get('/one-rms')).data).map(OneRm.fromJson).toList();

  Future<OneRm> setOneRm(String exerciseId, double value) async =>
      OneRm.fromJson(_map((await client.dio.put('/one-rms/$exerciseId', data: {'value': value})).data));

  Future<void> deleteOneRm(String exerciseId) => client.dio.delete('/one-rms/$exerciseId');

  // --- clubs ---

  Future<List<Club>> clubs() async =>
      _list((await client.dio.get('/clubs')).data).map(Club.fromJson).toList();

  // --- program authoring (coaches) ---

  Future<List<Program>> programs(String clubId) async =>
      _list((await client.dio.get('/clubs/$clubId/programs')).data).map(Program.fromJson).toList();

  /// The club's roster, for choosing who to hand a program to.
  Future<List<ClubMember>> clubMembers(String clubId) async =>
      _list((await client.dio.get('/clubs/$clubId/members')).data).map(ClubMember.fromJson).toList();

  /// Hands a program to one or more members in a single request.
  ///
  /// Plural because a coach starting a block runs it with a group: the API
  /// checks every member before writing any assignment, so a refusal leaves
  /// nobody half-assigned.
  Future<int> assignProgram(
    String clubId,
    String programId, {
    required List<String> userIds,
    String startDate = '',
    String note = '',
  }) async {
    final base = clubId.isEmpty ? '/programs' : '/clubs/$clubId/programs';
    final response = await client.dio.post('$base/$programId/assignments', data: {
      'userIds': userIds,
      if (startDate.isNotEmpty) 'startDate': startDate,
      if (note.isNotEmpty) 'note': note,
    });
    final data = response.data;
    return data is List ? data.length : 1;
  }

  /// Renames a program, rewrites its description and sets who may read it.
  ///
  /// All three go together because the API's PUT replaces them together: a
  /// payload that leaves the visibility out would republish a public program
  /// to its club, which is not what renaming it meant.
  Future<Program> updateProgram(
    String clubId,
    String programId, {
    required String name,
    required String description,
    required String visibility,
  }) async {
    final base = clubId.isEmpty ? '/programs' : '/clubs/$clubId/programs';
    final response = await client.dio.put('$base/$programId', data: {
      'name': name,
      'description': description,
      'visibility': visibility,
    });
    return Program.fromJson(_map(response.data));
  }

  Future<Program> createProgram(String clubId, String name, String description) async {
    final response = await client.dio
        .post('/clubs/$clubId/programs', data: {'name': name, 'description': description});
    return Program.fromJson(_map(response.data));
  }

  Future<ProgramDetail> program(String clubId, String programId) async =>
      ProgramDetail.fromJson(_map((await client.dio.get('/clubs/$clubId/programs/$programId')).data));

  Future<void> deleteProgram(String clubId, String programId) =>
      client.dio.delete('/clubs/$clubId/programs/$programId');

  /// Downloads the program as a document - a page or a sheet per week - and
  /// returns where it was saved.
  ///
  /// Rendered by the API rather than here: the loads on it are the ones the
  /// server computed, and a second implementation of the RPE chart on the
  /// phone could disagree with the screen it was exported from.
  ///
  /// [format] is 'pdf' or 'xlsx', and is the route's own extension - the file
  /// that arrives is named the way its contents are.
  Future<String> downloadProgram(
    String clubId,
    String programId, {
    required String directory,
    required String fileName,
    String format = 'pdf',
    String memberId = '',
    String locale = 'en',
  }) async {
    final base = clubId.isEmpty ? '/programs' : '/clubs/$clubId/programs';
    final path = '$directory/$fileName';
    await client.dio.download(
      '$base/$programId/export.$format',
      path,
      queryParameters: {
        if (memberId.isNotEmpty) 'memberId': memberId,
        'locale': locale,
      },
    );
    return path;
  }

  /// Downloads an assigned block with the member's feedback on it - the sheet
  /// a lifter sends their coach, and the one a coach reads away from the app.
  ///
  /// [week] limits it to that week; 0 is the whole block.
  Future<String> downloadAssignment(
    String assignmentId, {
    required String directory,
    required String fileName,
    String format = 'pdf',
    int week = 0,
    String locale = 'en',
  }) async {
    final path = '$directory/$fileName';
    await client.dio.download(
      '/training/$assignmentId/export.$format',
      path,
      queryParameters: {
        if (week > 0) 'week': week,
        'locale': locale,
      },
    );
    return path;
  }

  /// Leaving week/day at 0 continues the program's own numbering server-side,
  /// which is what adding sessions one after another wants.
  Future<ProgramDay> addDay(String clubId, String programId,
      {int week = 0, int day = 0, String title = ''}) async {
    final response = await client.dio.post('/clubs/$clubId/programs/$programId/days',
        data: {'week': week, 'day': day, 'title': title});
    return ProgramDay.fromJson(_map(response.data));
  }

  Future<void> deleteDay(String clubId, String programId, String dayId) =>
      client.dio.delete('/clubs/$clubId/programs/$programId/days/$dayId');

  Future<ProgramSet> saveSet(
    String clubId,
    String programId, {
    String? setId,
    String? dayId,
    required String exerciseId,
    required int reps,
    required String loadMode,
    double? rpe,
    double? percentage,
    double? absoluteLoad,
    String notes = '',
  }) async {
    // Only the field the mode uses is sent, so a set never carries a stale
    // number from a mode it isn't in.
    final data = {
      'exerciseId': exerciseId,
      'reps': reps,
      'loadMode': loadMode,
      'notes': notes,
      'rpe': loadMode == 'rpe' ? rpe : null,
      'percentage': loadMode == 'percentage' ? percentage : null,
      'absoluteLoad': loadMode == 'absolute' ? absoluteLoad : null,
    };
    final response = setId != null
        ? await client.dio.put('/clubs/$clubId/programs/$programId/sets/$setId', data: data)
        : await client.dio.post('/clubs/$clubId/programs/$programId/days/$dayId/sets', data: data);
    return ProgramSet.fromJson(_map(response.data));
  }

  Future<void> deleteSet(String clubId, String programId, String setId) =>
      client.dio.delete('/clubs/$clubId/programs/$programId/sets/$setId');

  // --- training ---

  Future<List<Assignment>> assignments() async =>
      _list((await client.dio.get('/training')).data).map(Assignment.fromJson).toList();

  Future<AssignmentDetail> assignment(String assignmentId) async =>
      AssignmentDetail.fromJson(_map((await client.dio.get('/training/$assignmentId')).data));

  Future<SetLog> logSet(
    String assignmentId,
    String setId, {
    int? actualReps,
    double? actualRpe,
    double? actualLoad,
    String comment = '',
    bool done = true,
  }) async {
    final response = await client.dio.put('/training/$assignmentId/sets/$setId/log', data: {
      'actualReps': actualReps,
      'actualRpe': actualRpe,
      'actualLoad': actualLoad,
      'comment': comment,
      'done': done,
    });
    return SetLog.fromJson(_map(response.data));
  }

  /// Exchanges the address of an object in a private bucket for one a player
  /// can actually open.
  ///
  /// The API serves those files itself and will not do so without knowing who
  /// is asking - which a video element cannot tell it. So the address in the
  /// post is exchanged here, with the session, for one carrying a short-lived
  /// signature.
  Future<String> mediaLink(String url) async {
    final path = url.replaceFirst(RegExp(r'^.*/v1'), '');
    final response = await client.dio.get('$path/link');
    return _rerootOnApi((_map(response.data)['url'] ?? '').toString());
  }

  /// Re-roots a signed playback link on the address this phone actually
  /// reaches.
  ///
  /// The API builds the link from its own `SF_API_URL` - whatever the server
  /// was told to call itself, which is not necessarily a name that resolves on
  /// the phone's network, and not necessarily how the member typed the address
  /// into the app. The signature covers the object, the viewer and the expiry
  /// and never the host, so moving the link onto the address the app is
  /// already talking to is safe, and is the difference between a video that
  /// plays and a blank frame.
  String _rerootOnApi(String signed) {
    final base = client.apiUrl;
    if (signed.isEmpty || base.isEmpty) return signed;
    final index = signed.indexOf('/v1/');
    return index < 0 ? signed : '$base${signed.substring(index)}';
  }

  /// Ticks a whole session off, or puts it back.
  ///
  /// One request rather than one per set: the API writes the flag onto every
  /// set of the day, keeping whatever each one already carried.
  Future<void> setDayDone(String assignmentId, String dayId, bool done) =>
      client.dio.put('/training/$assignmentId/days/$dayId/log', data: {'done': done});

  Future<Assignment> setAssignmentStatus(String assignmentId, String status) async => Assignment.fromJson(
      _map((await client.dio.put('/training/$assignmentId/status', data: {'status': status})).data));

  // --- social ---

  Future<Page<Post>> feed({int page = 0, bool discover = false}) async {
    final response = await client.dio.get(
      discover ? '/posts/discover' : '/posts',
      queryParameters: {'page': page, 'size': 20},
    );
    return Page.fromJson(_map(response.data), Post.fromJson);
  }

  /// Publishes a post. No links are sent: the API detects the first URL in
  /// the content and stores that, so there is one place a post's embed can
  /// come from and nothing for the two clients to disagree about.
  Future<Post> createPost({
    required String content,
    List<String> pictures = const [],
    String visibility = 'public',
    List<String> clubIds = const [],
  }) async {
    final response = await client.dio.post('/posts', data: {
      'content': content,
      'pictures': pictures,
      'visibility': visibility,
      'clubIds': clubIds,
    });
    return Post.fromJson(_map(response.data));
  }

  // --- administration (superadmin only) ---

  Future<Map<String, dynamic>> adminStats() async =>
      _map((await client.dio.get('/admin/stats')).data);

  Future<List<User>> adminUsers() async {
    final response = await client.dio.get('/admin/users');
    final data = response.data;
    if (data is! List) return const [];
    return data.map((item) => User.fromJson(_map(item))).toList();
  }

  /// Changes one account's global role. This is the whole of what the phone
  /// offers: activating somebody, promoting a coach, banning an abuser - the
  /// three things worth being able to do away from a desk.
  Future<User> adminSetRole(String userId, String role) async {
    final response = await client.dio.put('/admin/users/$userId', data: {'role': role});
    return User.fromJson(_map(response.data));
  }

  Future<void> adminDeleteUser(String userId) async {
    await client.dio.delete('/admin/users/$userId');
  }

  Future<List<ConnectionIp>> adminUserIps(String userId) async {
    final response = await client.dio.get('/admin/users/$userId/ips');
    final data = response.data;
    if (data is! List) return const [];
    return data.map((item) => ConnectionIp.fromJson(_map(item))).toList();
  }

  Future<List<CoachApplicant>> adminCoachRequests() async {
    final response = await client.dio.get('/admin/coach-requests');
    final data = response.data;
    if (data is! List) return const [];
    return data.map((item) => CoachApplicant.fromJson(_map(item))).toList();
  }

  Future<void> adminDecideCoachRequest(String userId, String status, String motive) async {
    await client.dio.put('/admin/coach-requests/$userId', data: {'status': status, 'motive': motive});
  }

  // --- media ---

  /// Uploads a video to the member's own storage and returns its URL.
  ///
  /// The file never lands in this app's database: the API forwards it to the
  /// bucket or Drive folder the member configured, and what comes back is a
  /// link. A member who has configured neither gets a 405, which surfaces as
  /// "set up your storage first" rather than as a failure.
  Future<String> uploadVideo(String path, {void Function(double)? onProgress}) {
    return _uploadMedia('/media/videos', path, onProgress: onProgress);
  }

  /// Uploads a recorded voice message. Separate from the video endpoint
  /// because the accepted types and the size cap differ.
  Future<String> uploadAudio(String path, {void Function(double)? onProgress}) {
    return _uploadMedia('/media/audio', path, onProgress: onProgress);
  }

  Future<String> _uploadMedia(String endpoint, String path, {void Function(double)? onProgress}) async {
    // The type has to be declared: the picker hands over a path, and dio sends
    // a part as application/octet-stream unless told otherwise - which the API
    // refused as "not something a browser can play", so every upload from the
    // phone failed while the same file uploaded fine from the web.
    final form = FormData.fromMap({
      'file': await MultipartFile.fromFile(path, contentType: _mediaTypeOf(path)),
    });
    final response = await client.dio.post(
      endpoint,
      data: form,
      // Generous on purpose: a video of a set, over whatever connection a gym
      // has, and the request is not done until the API has forwarded every
      // byte on to S3 or Drive. Ten minutes is far past any real upload and
      // still short enough that a dead connection reports something in the
      // end rather than spinning forever.
      options: Options(
        sendTimeout: const Duration(minutes: 10),
        receiveTimeout: const Duration(minutes: 10),
      ),
      onSendProgress: (sent, total) {
        if (total > 0) onProgress?.call(sent / total);
      },
    );
    final url = _map(response.data)['url'];
    if (url is! String || url.isEmpty) {
      // Returning an empty string here made a failed upload look like nothing
      // happened at all: the composer's guard dropped it silently, so the
      // member saw no link appear and no reason why.
      throw ApiException(
        i18nCode: 'errors.storageUploadFailed',
        statusCode: response.statusCode,
      );
    }
    return url;
  }

  /// The content type for a file, read off its extension.
  ///
  /// The list is the one the API accepts (its videoContentTypes and
  /// audioContentTypes); anything else is sent unlabelled and refused there,
  /// which is the right place for that answer to come from.
  MediaType? _mediaTypeOf(String path) {
    const types = {
      '.mp4': ['video', 'mp4'],
      '.webm': ['video', 'webm'],
      '.ogv': ['video', 'ogg'],
      '.mov': ['video', 'quicktime'],
      '.m4a': ['audio', 'mp4'],
      '.mp3': ['audio', 'mpeg'],
      '.aac': ['audio', 'aac'],
      '.wav': ['audio', 'wav'],
      '.ogg': ['audio', 'ogg'],
    };

    final dot = path.lastIndexOf('.');
    if (dot < 0) return null;
    final parts = types[path.substring(dot).toLowerCase()];
    return parts == null ? null : MediaType(parts[0], parts[1]);
  }

  // --- private messages ---

  Future<List<Conversation>> conversations() async {
    final response = await client.dio.get('/messages');
    final data = response.data;
    if (data is! List) return const [];
    return data.map((item) => Conversation.fromJson(_map(item))).toList();
  }

  Future<int> unreadMessages() async {
    final response = await client.dio.get('/messages/unread');
    final value = _map(response.data)['unread'];
    return value is num ? value.toInt() : int.tryParse('$value') ?? 0;
  }

  /// Opens the thread with one member, which also marks it read.
  Future<Thread> thread(String userId) async {
    final response = await client.dio.get('/messages/with/$userId');
    return Thread.fromJson(_map(response.data));
  }

  Future<PrivateMessage> sendMessage(
    String userId, {
    String content = '',
    List<String> pictures = const [],
    String audio = '',
  }) async {
    final response = await client.dio.post('/messages/with/$userId', data: {
      'content': content,
      'pictures': pictures,
      'audio': audio,
    });
    return PrivateMessage.fromJson(_map(response.data));
  }

  /// Removes one private message. Its sender may take back what they wrote;
  /// a superadmin may remove anything. The API decides, and reports it as
  /// `deletable` on each message.
  Future<void> deleteMessage(String messageId) async {
    await client.dio.delete('/messages/$messageId');
  }

  Future<void> blockMember(String userId) async {
    await client.dio.post('/blocks/$userId');
  }

  // --- invitations ---

  Future<List<Invitation>> invitations() async {
    final response = await client.dio.get('/users/me/invitations');
    final data = response.data;
    if (data is! List) return const [];
    return data.map((item) => Invitation.fromJson(_map(item))).toList();
  }

  Future<void> acceptInvitation(String id) async {
    await client.dio.post('/users/me/invitations/$id/accept');
  }

  Future<void> declineInvitation(String id) async {
    await client.dio.post('/users/me/invitations/$id/decline');
  }

  // --- the calendar ---

  /// The events this member may see: the open calendar plus their own clubs'
  /// dates. Readable logged out, though the app always has a session by the
  /// time it asks.
  Future<List<Event>> events({bool past = false}) async {
    final response = await client.dio.get('/events', queryParameters: past ? {'past': 1} : null);
    final data = response.data;
    if (data is! List) return const [];
    return data.map((item) => Event.fromJson(_map(item))).toList();
  }

  Future<Post> like(String postId, bool liked) async {
    final response =
        liked ? await client.dio.delete('/posts/$postId/like') : await client.dio.post('/posts/$postId/like');
    return Post.fromJson(_map(response.data));
  }

  Future<void> deletePost(String postId) => client.dio.delete('/posts/$postId');

  /// Moves a post between the public feed and one of its author's clubs.
  ///
  /// The content and pictures go with it because the API rewrites the whole
  /// post: sending only the visibility would blank the text. Links are left
  /// out on purpose - the API re-derives them from the content.
  /// Rewrites a post's text, keeping everything else as it is.
  ///
  /// The whole post is sent because the API replaces the payload rather than
  /// patching it: leaving the pictures and the clubs out would clear them.
  Future<Post> updatePost(Post post, String content) async {
    final response = await client.dio.put('/posts/${post.id}', data: {
      'content': content,
      'pictures': post.pictures,
      'visibility': post.visibility,
      'clubIds': post.clubIds,
    });
    return Post.fromJson(_map(response.data));
  }

  Future<Post> updatePostVisibility(Post post, String visibility, List<String> clubIds) async {
    final response = await client.dio.put('/posts/${post.id}', data: {
      'content': post.content,
      'pictures': post.pictures,
      'visibility': visibility,
      'clubIds': visibility == 'club' ? clubIds : const <String>[],
    });
    return Post.fromJson(_map(response.data));
  }

  Future<Page<Comment>> comments(String postId, {int page = 0}) async {
    final response =
        await client.dio.get('/posts/$postId/comments', queryParameters: {'page': page, 'size': 20});
    return Page.fromJson(_map(response.data), Comment.fromJson);
  }

  Future<Comment> addComment(String postId, String content) async => Comment.fromJson(
      _map((await client.dio.post('/posts/$postId/comments', data: {'content': content})).data));

  /// Rewrites a comment. Allowed for its author and for a superadmin, which is
  /// what the comment's own `editable` flag reports.
  Future<Comment> updateComment(String postId, String commentId, String content) async =>
      Comment.fromJson(_map(
        (await client.dio.put('/posts/$postId/comments/$commentId', data: {'content': content})).data,
      ));

  Future<void> deleteComment(String postId, String commentId) =>
      client.dio.delete('/posts/$postId/comments/$commentId');

  Future<void> report({
    required String targetType,
    required String targetId,
    required String reason,
    String comment = '',
  }) =>
      client.dio.post('/reports', data: {
        'targetType': targetType,
        'targetId': targetId,
        'reason': reason,
        'comment': comment,
      });

  // --- profiles ---

  /// Finds members, the way sf-ui's search page does: a free-text `terms`
  /// plus the fields you can narrow by, combined with AND.
  ///
  /// Nothing is filtered here. Who may be seen is decided inside the API's
  /// query, so the count that comes back is honest and a hidden profile was
  /// never in it. With no criteria at all it answers with everybody the caller
  /// may see, which is what makes the screen useful before anything is typed.
  Future<Page<MemberResult>> searchMembers({
    String terms = '',
    String name = '',
    String surname = '',
    String username = '',
    String email = '',
    int page = 0,
  }) async {
    final criteria = {
      'terms': terms,
      'name': name,
      'surname': surname,
      'username': username,
      'email': email,
    }..removeWhere((_, value) => value.trim().isEmpty);

    final response = await client.dio.get('/search/members', queryParameters: {
      for (final entry in criteria.entries) entry.key: entry.value.trim(),
      'page': page,
      'size': 20,
    });
    return Page.fromJson(_map(response.data), MemberResult.fromJson);
  }

  Future<PublicProfile> profile(String handle) async =>
      PublicProfile.fromJson(_map((await client.dio.get('/profiles/$handle')).data));

  Future<Page<Post>> profilePosts(String handle, {int page = 0}) async {
    final response =
        await client.dio.get('/profiles/$handle/posts', queryParameters: {'page': page, 'size': 20});
    return Page.fromJson(_map(response.data), Post.fromJson);
  }

  Future<void> follow(String handle, bool following) => following
      ? client.dio.delete('/profiles/$handle/follow')
      : client.dio.post('/profiles/$handle/follow');
}
