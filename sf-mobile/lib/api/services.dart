import 'api_client.dart';
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

  Future<Program> createProgram(String clubId, String name, String description) async {
    final response = await client.dio
        .post('/clubs/$clubId/programs', data: {'name': name, 'description': description});
    return Program.fromJson(_map(response.data));
  }

  Future<ProgramDetail> program(String clubId, String programId) async =>
      ProgramDetail.fromJson(_map((await client.dio.get('/clubs/$clubId/programs/$programId')).data));

  Future<void> deleteProgram(String clubId, String programId) =>
      client.dio.delete('/clubs/$clubId/programs/$programId');

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

  Future<void> clearLog(String assignmentId, String setId) =>
      client.dio.delete('/training/$assignmentId/sets/$setId/log');

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
    String clubId = '',
  }) async {
    final response = await client.dio.post('/posts', data: {
      'content': content,
      'pictures': pictures,
      'visibility': visibility,
      'clubId': clubId,
    });
    return Post.fromJson(_map(response.data));
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

  Future<PrivateMessage> sendMessage(String userId, String content) async {
    final response = await client.dio.post('/messages/with/$userId', data: {'content': content});
    return PrivateMessage.fromJson(_map(response.data));
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

  Future<Page<Comment>> comments(String postId, {int page = 0}) async {
    final response =
        await client.dio.get('/posts/$postId/comments', queryParameters: {'page': page, 'size': 20});
    return Page.fromJson(_map(response.data), Comment.fromJson);
  }

  Future<Comment> addComment(String postId, String content) async => Comment.fromJson(
      _map((await client.dio.post('/posts/$postId/comments', data: {'content': content})).data));

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
